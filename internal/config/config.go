package config

import "time"

type Config struct {
	ExporterPort      int            `mapstructure:"exporter_port" validate:"required,min=1,max=65535"`
	PollingInterval   time.Duration  `mapstructure:"polling_interval" validate:"required,min=1s"`
	Metrics           []MetricConfig `mapstructure:"metrics" validate:"required,min=1,dive"`
	TargetAWSAccounts []AWSAccount   `mapstructure:"target_aws_accounts" validate:"required,min=1"`
}

type MetricConfig struct {
	MetricName        string         `mapstructure:"metric_name" validate:"required"`
	MetricDescription string         `mapstructure:"metric_description"`
	Granularity       string         `mapstructure:"granularity" validate:"required,oneof=DAILY MONTHLY"`
	DataDelayDays     int            `mapstructure:"data_delay_days" validate:"min=0"`
	MetricType        string         `mapstructure:"metric_type" validate:"required"`
	RecordTypes       []string       `mapstructure:"record_types"`
	GroupBy           *GroupByConfig `mapstructure:"group_by"`
	TagFilters        []TagFilter    `mapstructure:"tag_filters"`
}

type GroupByConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	Groups         []GroupConfig `mapstructure:"groups" validate:"max=2"`
	MergeMinorCost *MergeConfig  `mapstructure:"merge_minor_cost"`
}

type GroupConfig struct {
	Type      string       `mapstructure:"type" validate:"required,oneof=DIMENSION TAG COST_CATEGORY"`
	Key       string       `mapstructure:"key" validate:"required"`
	LabelName string       `mapstructure:"label_name" validate:"required"`
	Alias     *AliasConfig `mapstructure:"alias"`
}

type AliasConfig struct {
	LabelName string            `mapstructure:"label_name" validate:"required"`
	Map       map[string]string `mapstructure:"map"`
}

type MergeConfig struct {
	Enabled   bool    `mapstructure:"enabled"`
	Threshold float64 `mapstructure:"threshold"`
	TagValue  string  `mapstructure:"tag_value"`
}

type TagFilter struct {
	TagKey    string   `mapstructure:"tag_key" validate:"required"`
	TagValues []string `mapstructure:"tag_values" validate:"required,min=1"`
}

type AWSAccount struct {
	AccountId       string            `mapstructure:"account_id" validate:"required"`
	AssumedRoleName string            `mapstructure:"assumed_role_name" validate:"required"`
	Labels          map[string]string `mapstructure:"labels"`
}
