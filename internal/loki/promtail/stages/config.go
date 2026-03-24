package stages

import "github.com/grafana/alloy/internal/loki/util/flagext"

const (
	StageTypeJSON               = "json"
	StageTypeLogfmt             = "logfmt"
	StageTypeRegex              = "regex"
	StageTypeReplace            = "replace"
	StageTypeMetric             = "metrics"
	StageTypeLabel              = "labels"
	StageTypeLabelDrop          = "labeldrop"
	StageTypeTimestamp          = "timestamp"
	StageTypeOutput             = "output"
	StageTypeDocker             = "docker"
	StageTypeCRI                = "cri"
	StageTypeMatch              = "match"
	StageTypeTemplate           = "template"
	StageTypePipeline           = "pipeline"
	StageTypeTenant             = "tenant"
	StageTypeDrop               = "drop"
	StageTypeSampling           = "sampling"
	StageTypeLimit              = "limit"
	StageTypeMultiline          = "multiline"
	StageTypePack               = "pack"
	StageTypeLabelAllow         = "labelallow"
	StageTypeStaticLabels       = "static_labels"
	StageTypeDecolorize         = "decolorize"
	StageTypeEventLogMessage    = "eventlogmessage"
	StageTypeGeoIP              = "geoip"
	StageTypeStructuredMetadata = "structured_metadata"
)

// PipelineStages contains configuration for each stage within a pipeline
type PipelineStages = []any

// PipelineStage contains configuration for a single pipeline stage
type PipelineStage = map[any]any

// DropConfig contains the configuration for a dropStage
type DropConfig struct {
	DropReason *string `mapstructure:"drop_counter_reason"`
	Source     any     `mapstructure:"source"`
	Value      *string `mapstructure:"value"`
	Separator  *string `mapstructure:"separator"`
	Expression *string `mapstructure:"expression"`
	OlderThan  *string `mapstructure:"older_than"`
	LongerThan *string `mapstructure:"longer_than"`
}

type EventLogMessageConfig struct {
	Source            *string `mapstructure:"source"`
	DropInvalidLabels bool    `mapstructure:"drop_invalid_labels"`
	OverwriteExisting bool    `mapstructure:"overwrite_existing"`
}

const MaxPartialLinesSize = 100 // Max buffer size to hold partial lines.

// CriConfig contains the configuration for the cri stage
type CriConfig struct {
	MaxPartialLines            int              `mapstructure:"max_partial_lines"`
	MaxPartialLineSize         flagext.ByteSize `mapstructure:"max_partial_line_size"`
	MaxPartialLineSizeTruncate bool             `mapstructure:"max_partial_line_size_truncate"`
}

// GeoIPConfig represents GeoIP stage config
type GeoIPConfig struct {
	DB     string  `mapstructure:"db"`
	Source *string `mapstructure:"source"`
	DBType string  `mapstructure:"db_type"`
}

// JSONConfig represents a JSON Stage configuration
type JSONConfig struct {
	Expressions   map[string]string `mapstructure:"expressions"`
	Source        *string           `mapstructure:"source"`
	DropMalformed bool              `mapstructure:"drop_malformed"`
}

// labelallowConfig is a slice of labels to be included
type LabelAllowConfig []string

// LabelDropConfig is a slice of labels to be dropped
type LabelDropConfig []string

// LabelsConfig is a set of labels to be extracted
type LabelsConfig map[string]*string

type LimitConfig struct {
	Rate              float64 `mapstructure:"rate"`
	Burst             int     `mapstructure:"burst"`
	Drop              bool    `mapstructure:"drop"`
	ByLabelName       string  `mapstructure:"by_label_name"`
	MaxDistinctLabels int     `mapstructure:"max_distinct_labels"`
}

// LogfmtConfig represents a logfmt Stage configuration
type LogfmtConfig struct {
	Mapping map[string]string `mapstructure:"mapping"`
	Source  *string           `mapstructure:"source"`
}

// MatcherConfig contains the configuration for a matcherStage
type MatcherConfig struct {
	PipelineName *string        `mapstructure:"pipeline_name"`
	Selector     string         `mapstructure:"selector"`
	Stages       PipelineStages `mapstructure:"stages"`
	Action       string         `mapstructure:"action"`
	DropReason   *string        `mapstructure:"drop_counter_reason"`
}

// MetricConfig is a single metrics configuration.
type MetricConfig struct {
	MetricType   string  `mapstructure:"type"`
	Description  string  `mapstructure:"description"`
	Source       *string `mapstructure:"source"`
	Prefix       string  `mapstructure:"prefix"`
	IdleDuration *string `mapstructure:"max_idle_duration"`
	Config       any     `mapstructure:"config"`
}

const (
	MetricTypeCounter   = "counter"
	MetricTypeGauge     = "gauge"
	MetricTypeHistogram = "histogram"
)

// MetricsConfig is a set of configured metrics.
type MetricsConfig map[string]MetricConfig

// MultilineConfig contains the configuration for a multilineStage
type MultilineConfig struct {
	Expression  *string `mapstructure:"firstline"`
	MaxLines    *uint64 `mapstructure:"max_lines"`
	MaxWaitTime *string `mapstructure:"max_wait_time"`
}

// OutputConfig configures output value extraction
type OutputConfig struct {
	Source string `mapstructure:"source"`
}

// PackConfig contains the configuration for a packStage
type PackConfig struct {
	Labels          []string `mapstrcuture:"labels"`
	IngestTimestamp *bool    `mapstructure:"ingest_timestamp"`
}

// RegexConfig contains a regexStage configuration
type RegexConfig struct {
	Expression string  `mapstructure:"expression"`
	Source     *string `mapstructure:"source"`
}

// ReplaceConfig contains a regexStage configuration
type ReplaceConfig struct {
	Expression string  `mapstructure:"expression"`
	Source     *string `mapstructure:"source"`
	Replace    string  `mapstructure:"replace"`
}

// SamplingConfig contains the configuration for a samplingStage
type SamplingConfig struct {
	DropReason   *string `mapstructure:"drop_counter_reason"`
	SamplingRate float64 `mapstructure:"rate"`
}

// StaticLabelConfig is a slice of static-labels to be included
type StaticLabelConfig map[string]*string

// TemplateConfig configures template value extraction
type TemplateConfig struct {
	Source   string `mapstructure:"source"`
	Template string `mapstructure:"template"`
}

// TenantConfig configures tenant extraction
type TenantConfig struct {
	Label  string `mapstructure:"label"`
	Source string `mapstructure:"source"`
	Value  string `mapstructure:"value"`
}

// TimestampConfig configures timestamp extraction
type TimestampConfig struct {
	Source          string   `mapstructure:"source"`
	Format          string   `mapstructure:"format"`
	FallbackFormats []string `mapstructure:"fallback_formats"`
	Location        *string  `mapstructure:"location"`
	ActionOnFailure *string  `mapstructure:"action_on_failure"`
}
