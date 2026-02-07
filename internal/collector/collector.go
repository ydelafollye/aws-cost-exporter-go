package collector

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ydelafollye/aws-cost-exporter-go/internal/aws"
	"github.com/ydelafollye/aws-cost-exporter-go/internal/config"
	"github.com/ydelafollye/aws-cost-exporter-go/pkg/timeutil"
)

type CostCollector struct {
	mu         sync.RWMutex
	metrics    map[string]*prometheus.GaugeVec
	awsClients map[string]*aws.CostExplorerClient
	config     *config.Config
	logger     *slog.Logger

	// Internal metrics
	scrapeErrors   prometheus.Counter
	scrapeDuration prometheus.Histogram
}

func New(cfg *config.Config, logger *slog.Logger) (*CostCollector, error) {
	c := &CostCollector{
		metrics:    make(map[string]*prometheus.GaugeVec),
		awsClients: make(map[string]*aws.CostExplorerClient),
		config:     cfg,
		logger:     logger,
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "aws_cost_exporter_scrape_errors_total",
			Help: "Total number of scrape errors",
		}),
		scrapeDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "aws_cost_exporter_scrape_duration_seconds",
			Help:    "Duration of cost data scraping",
			Buckets: prometheus.DefBuckets,
		}),
	}

	// Init metrics from config
	for _, metricCfg := range cfg.Metrics {
		labels := buildLabelNames(cfg, &metricCfg)
		c.metrics[metricCfg.MetricName] = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricCfg.MetricName,
				Help: metricCfg.MetricDescription,
			},
			labels,
		)
	}

	// Init clients for each AWS account
	for _, account := range cfg.TargetAWSAccounts {
		client, err := aws.NewCostExplorerClient(cfg, account.AccountId, account.AssumedRoleName)
		if err != nil {
			return nil, fmt.Errorf("creating AWS client for %s: %w", account.AccountId, err)
		}
		c.awsClients[account.AccountId] = client
	}

	return c, nil

}

// Implement prometheus.Describe
func (c *CostCollector) Describe(ch chan<- *prometheus.Desc) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, metric := range c.metrics {
		metric.Describe(ch)
	}
	c.scrapeErrors.Describe(ch)
	c.scrapeDuration.Describe(ch)
}

// Implement prometheus.Collector
func (c *CostCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, metric := range c.metrics {
		metric.Collect(ch)
	}
	c.scrapeErrors.Collect(ch)
	c.scrapeDuration.Collect(ch)
}

// accountResults holds the fetched results for one account
type accountResults struct {
	account config.AWSAccount
	results map[string]*aws.CostResult // metric name -> result
}

// Get data from all accounts (called by the poller)
func (c *CostCollector) Refresh(ctx context.Context) error {
	timer := prometheus.NewTimer(c.scrapeDuration)
	defer timer.ObserveDuration()

	// Fetch all accounts in parallel (without holding the lock)
	var wg sync.WaitGroup
	resultsCh := make(chan accountResults, len(c.config.TargetAWSAccounts))
	errCh := make(chan error, len(c.config.TargetAWSAccounts))

	for _, account := range c.config.TargetAWSAccounts {
		wg.Add(1)
		go func(acc config.AWSAccount) {
			defer wg.Done()
			results, err := c.fetchAccountCosts(ctx, acc)
			if err != nil {
				c.logger.Error("failed to fetch costs",
					"account", acc.AccountId,
					"error", err)
				c.scrapeErrors.Inc()
				errCh <- err
				return
			}
			resultsCh <- accountResults{account: acc, results: results}
		}(account)
	}

	wg.Wait()
	close(resultsCh)
	close(errCh)

	// Collect all results
	var allResults []accountResults
	for r := range resultsCh {
		allResults = append(allResults, r)
	}

	// Now atomically reset and update all metrics
	c.mu.Lock()
	for _, metric := range c.metrics {
		metric.Reset()
	}
	for _, ar := range allResults {
		for _, metricCfg := range c.config.Metrics {
			if result, ok := ar.results[metricCfg.MetricName]; ok {
				c.updateMetrics(ar.account, &metricCfg, result)
			}
		}
	}
	c.mu.Unlock()

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d accounts failed to fetch", len(errs))
	}

	return nil
}

func (c *CostCollector) fetchAccountCosts(ctx context.Context, account config.AWSAccount) (map[string]*aws.CostResult, error) {
	client, ok := c.awsClients[account.AccountId]
	if !ok {
		return nil, fmt.Errorf("no client found for account %s", account.AccountId)
	}

	results := make(map[string]*aws.CostResult)
	for _, metricCfg := range c.config.Metrics {
		query := buildQuery(&metricCfg)
		result, err := client.GetCostAndUsage(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("metric %s: %w", metricCfg.MetricName, err)
		}
		results[metricCfg.MetricName] = result
	}

	return results, nil
}

func buildQuery(metricCfg *config.MetricConfig) *aws.CostQuery {
	var period timeutil.Period
	if metricCfg.Granularity == "DAILY" {
		period = timeutil.DailyPeriod(metricCfg.DataDelayDays)
	} else {
		period = timeutil.MonthlyPeriod(metricCfg.DataDelayDays)
	}

	var groupBy []types.GroupDefinition
	if metricCfg.GroupBy != nil && metricCfg.GroupBy.Enabled {
		for _, g := range metricCfg.GroupBy.Groups {
			groupBy = append(groupBy, types.GroupDefinition{
				Type: types.GroupDefinitionType(g.Type),
				Key:  awssdk.String(g.Key),
			})
		}
	}

	return &aws.CostQuery{
		StartDate:   period.Start,
		EndDate:     period.End,
		Granularity: metricCfg.Granularity,
		MetricType:  metricCfg.MetricType,
		RecordTypes: metricCfg.RecordTypes,
		GroupBy:     groupBy,
		TagFilters:  metricCfg.TagFilters,
	}
}

func (c *CostCollector) updateMetrics(account config.AWSAccount, metricCfg *config.MetricConfig, result *aws.CostResult) {
	gauge := c.metrics[metricCfg.MetricName]

	if metricCfg.GroupBy == nil || !metricCfg.GroupBy.Enabled {
		labels := buildLabelValues(account, metricCfg, nil)
		gauge.WithLabelValues(labels...).Set(result.Total)
		return
	}

	var mergedMinorCost float64
	mergeEnabled := metricCfg.GroupBy.MergeMinorCost != nil &&
		metricCfg.GroupBy.MergeMinorCost.Enabled

	for _, group := range result.Groups {
		if mergeEnabled && group.Amount < metricCfg.GroupBy.MergeMinorCost.Threshold {
			mergedMinorCost += group.Amount
			continue
		}

		labels := buildLabelValues(account, metricCfg, group.Keys)
		gauge.WithLabelValues(labels...).Set(group.Amount)
	}

	if mergedMinorCost > 0 {
		mergedKeys := make([]string, len(metricCfg.GroupBy.Groups))
		for i := range mergedKeys {
			mergedKeys[i] = metricCfg.GroupBy.MergeMinorCost.TagValue
		}
		labels := buildLabelValues(account, metricCfg, mergedKeys)
		gauge.WithLabelValues(labels...).Set(mergedMinorCost)
	}
}
