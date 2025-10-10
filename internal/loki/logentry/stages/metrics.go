package stages

const (
	MetricTypeCounter   = "counter"
	MetricTypeGauge     = "gauge"
	MetricTypeHistogram = "histogram"

	ErrEmptyMetricsStageConfig = "empty metric stage configuration"
	ErrMetricsStageInvalidType = "invalid metric type '%s', metric type must be one of 'counter', 'gauge', or 'histogram'"
	ErrInvalidIdleDur          = "max_idle_duration could not be parsed as a time.Duration: '%s'"
	ErrSubSecIdleDur           = "max_idle_duration less than 1s not allowed"
)

// MetricConfig is a single metrics configuration.
type MetricConfig struct {
	MetricType   string      `mapstructure:"type"`
	Description  string      `mapstructure:"description"`
	Source       *string     `mapstructure:"source"`
	Prefix       string      `mapstructure:"prefix"`
	IdleDuration *string     `mapstructure:"max_idle_duration"`
	Config       interface{} `mapstructure:"config"`
}

// MetricsConfig is a set of configured metrics.
type MetricsConfig map[string]MetricConfig
