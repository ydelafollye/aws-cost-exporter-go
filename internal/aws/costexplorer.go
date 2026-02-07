package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/ydelafollye/aws-cost-exporter-go/internal/config"
)

type CostExplorerClient struct {
	client *costexplorer.Client
}

type CostQuery struct {
	StartDate   time.Time
	EndDate     time.Time
	Granularity string
	MetricType  string
	RecordTypes []string
	GroupBy     []types.GroupDefinition
	TagFilters  []config.TagFilter
}

type CostResult struct {
	Groups []CostGroup
	Total  float64
}

type CostGroup struct {
	Keys   []string
	Amount float64
	Unit   string
}

func buildFilter(recordTypes []string, tagFilters []config.TagFilter) *types.Expression {
	if len(recordTypes) == 0 {
		recordTypes = []string{"Usage"}
	}

	baseFilter := &types.Expression{
		Dimensions: &types.DimensionValues{
			Key:    types.DimensionRecordType,
			Values: recordTypes,
		},
	}

	if len(tagFilters) == 0 {
		return baseFilter
	}

	var allFilters []types.Expression
	allFilters = append(allFilters, *baseFilter)

	for _, tf := range tagFilters {
		tagFilter := types.Expression{
			Tags: &types.TagValues{
				Key:          aws.String(tf.TagKey),
				Values:       tf.TagValues,
				MatchOptions: []types.MatchOption{types.MatchOptionEquals},
			},
		}
		allFilters = append(allFilters, tagFilter)
	}

	return &types.Expression{
		And: allFilters,
	}
}

func NewCostExplorerClient(cfg *config.Config, accountId string, assumedRoleName string) (*CostExplorerClient, error) {
	ctx := context.Background()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(awsCfg)
	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", accountId, assumedRoleName)
	creds := stscreds.NewAssumeRoleProvider(stsClient, roleARN)
	awsCfg.Credentials = aws.NewCredentialsCache(creds)

	return &CostExplorerClient{
		client: costexplorer.NewFromConfig(awsCfg),
	}, nil
}

func (c *CostExplorerClient) GetCostAndUsage(ctx context.Context, query *CostQuery) (*CostResult, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(query.StartDate.Format("2006-01-02")),
			End:   aws.String(query.EndDate.Format("2006-01-02")),
		},
		Granularity: types.Granularity(query.Granularity),
		Metrics:     []string{query.MetricType},
		GroupBy:     query.GroupBy,
		Filter:      buildFilter(query.RecordTypes, query.TagFilters),
	}

	var result CostResult

	for {
		page, err := c.client.GetCostAndUsage(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("fetching cost data: %w", err)
		}

		for _, resultByTime := range page.ResultsByTime {
			// Handle grouped results
			for _, group := range resultByTime.Groups {
				metric, ok := group.Metrics[query.MetricType]
				if !ok || metric.Amount == nil {
					continue
				}
				amount, err := strconv.ParseFloat(*metric.Amount, 64)
				if err != nil {
					return nil, fmt.Errorf("parsing cost amount %q: %w", *metric.Amount, err)
				}
				unit := ""
				if metric.Unit != nil {
					unit = *metric.Unit
				}
				result.Groups = append(result.Groups, CostGroup{
					Keys:   group.Keys,
					Amount: amount,
					Unit:   unit,
				})
			}

			// Handle ungrouped results (Total)
			if len(resultByTime.Groups) == 0 && resultByTime.Total != nil {
				if metric, ok := resultByTime.Total[query.MetricType]; ok && metric.Amount != nil {
					amount, err := strconv.ParseFloat(*metric.Amount, 64)
					if err != nil {
						return nil, fmt.Errorf("parsing total amount %q: %w", *metric.Amount, err)
					}
					result.Total += amount
				}
			}
		}

		if page.NextPageToken == nil {
			break
		}
		input.NextPageToken = page.NextPageToken
	}

	return &result, nil
}
