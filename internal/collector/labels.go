package collector

import (
	"sort"
	"strings"

	"github.com/ydelafollye/aws-cost-exporter-go/internal/config"
)

// getSortedLabelKeys returns sorted keys from account labels for deterministic order
func getSortedLabelKeys(account config.AWSAccount) []string {
	keys := make([]string, 0, len(account.Labels))
	for k := range account.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// getChargeType returns the charge type value based on RecordTypes config
func getChargeType(metricCfg *config.MetricConfig) string {
	if len(metricCfg.RecordTypes) == 0 {
		return "Usage"
	}
	return strings.Join(metricCfg.RecordTypes, ",")
}

func buildLabelNames(cfg *config.Config, metricCfg *config.MetricConfig) []string {
	var labels []string

	if len(cfg.TargetAWSAccounts) > 0 {
		labels = append(labels, "account_id")
		labels = append(labels, getSortedLabelKeys(cfg.TargetAWSAccounts[0])...)
	}

	labels = append(labels, "charge_type")

	if metricCfg.GroupBy != nil && metricCfg.GroupBy.Enabled {
		for _, group := range metricCfg.GroupBy.Groups {
			labels = append(labels, group.LabelName)
			if group.Alias != nil {
				labels = append(labels, group.Alias.LabelName)
			}
		}
	}

	return labels
}

func buildLabelValues(account config.AWSAccount, metricCfg *config.MetricConfig, keys []string) []string {
	var values []string
	values = append(values, account.AccountId)

	for _, key := range getSortedLabelKeys(account) {
		values = append(values, account.Labels[key])
	}

	values = append(values, getChargeType(metricCfg))

	if metricCfg.GroupBy != nil && metricCfg.GroupBy.Enabled {
		for i, group := range metricCfg.GroupBy.Groups {
			keyValue := ""
			if i < len(keys) {
				keyValue = keys[i]
			}
			// strip the prefix to keep only the tag value.
			if group.Type == "TAG" {
				if prefix := group.Key + "$"; strings.HasPrefix(keyValue, prefix) {
					keyValue = strings.TrimPrefix(keyValue, prefix)
				}
			}
			values = append(values, keyValue)
			if group.Alias != nil {
				aliasValue := keyValue
				if keyValue != "" {
					if mapped, ok := group.Alias.Map[keyValue]; ok {
						aliasValue = mapped
					}
				}
				values = append(values, aliasValue)
			}
		}
	}

	return values
}
