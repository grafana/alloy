package automaticloggingprocessor

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/static/logs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

// TypeStr is the unique identifier for the Automatic Logging processor.
const TypeStr = "automatic_logging"

// Config holds the configuration for the Automatic Logging processor.
type Config struct {
	LoggingConfig *AutomaticLoggingConfig `mapstructure:"automatic_logging"`
}

// AutomaticLoggingConfig holds config information for automatic logging
type AutomaticLoggingConfig struct {
	Backend           string         `mapstructure:"backend" yaml:"backend,omitempty"`
	LogsName          string         `mapstructure:"logs_instance_name" yaml:"logs_instance_name,omitempty"`
	Spans             bool           `mapstructure:"spans" yaml:"spans,omitempty"`
	Roots             bool           `mapstructure:"roots" yaml:"roots,omitempty"`
	Processes         bool           `mapstructure:"processes" yaml:"processes,omitempty"`
	SpanAttributes    []string       `mapstructure:"span_attributes" yaml:"span_attributes,omitempty"`
	ProcessAttributes []string       `mapstructure:"process_attributes" yaml:"process_attributes,omitempty"`
	Overrides         OverrideConfig `mapstructure:"overrides" yaml:"overrides,omitempty"`
	Timeout           time.Duration  `mapstructure:"timeout" yaml:"timeout,omitempty"`
	Labels            []string       `mapstructure:"labels" yaml:"labels,omitempty"`

	// Deprecated fields:
	LokiName string `mapstructure:"loki_name" yaml:"loki_name,omitempty"` // Superseded by LogsName
}

// Validate ensures that the AutomaticLoggingConfig is valid.
func (c *AutomaticLoggingConfig) Validate(logsConfig *logs.Config) error {
	if c.Backend == BackendLoki {
		c.Backend = BackendLogs
	}

	if c.LogsName != "" && c.LokiName != "" {
		return fmt.Errorf("must configure at most one of logs_instance_name and loki_name. loki_name is deprecated in favor of logs_instance_name")
	}

	if c.LogsName != "" && logsConfig == nil {
		return fmt.Errorf("logs instance %s is set but no logs config is provided", c.LogsName)
	}

	// Migrate deprecated config to new one
	if c.LogsName == "" && c.LokiName != "" {
		c.LogsName, c.LokiName = c.LokiName, ""
	}

	if c.Overrides.LogsTag != "" && c.Overrides.LokiTag != "" {
		return fmt.Errorf("must configure at most one of overrides.logs_instance_tag and overrides.loki_tag. loki_tag is deprecated in favor of logs_instance_tag")
	}

	// Migrate deprecated config to new one
	if c.Overrides.LogsTag == "" && c.Overrides.LokiTag != "" {
		c.Overrides.LogsTag, c.Overrides.LokiTag = c.Overrides.LokiTag, ""
	}

	// Ensure the logging instance exists when using it as a backend.
	if c.Backend == BackendLogs {
		var found bool
		for _, inst := range logsConfig.Configs {
			if inst.Name == c.LogsName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("specified logs config %s not found in agent config", c.LogsName)
		}
	}

	return nil
}

// OverrideConfig contains overrides for various strings
type OverrideConfig struct {
	LogsTag     string `mapstructure:"logs_instance_tag" yaml:"logs_instance_tag,omitempty"`
	ServiceKey  string `mapstructure:"service_key" yaml:"service_key,omitempty"`
	SpanNameKey string `mapstructure:"span_name_key" yaml:"span_name_key,omitempty"`
	StatusKey   string `mapstructure:"status_key" yaml:"status_key,omitempty"`
	DurationKey string `mapstructure:"duration_key" yaml:"duration_key,omitempty"`
	TraceIDKey  string `mapstructure:"trace_id_key" yaml:"trace_id_key,omitempty"`

	// Deprecated fields:
	LokiTag string `mapstructure:"loki_tag" yaml:"loki_tag,omitempty"` // Superseded by LogsTag
}

const (
	// BackendLogs is the backend config for sending logs to a Loki pipeline
	BackendLogs = "logs_instance"
	// BackendLoki is an alias to BackendLogs. DEPRECATED.
	BackendLoki = "loki"
	// BackendStdout is the backend config value for sending logs to stdout
	BackendStdout = "stdout"
)

// NewFactory returns a new factory for the Attributes processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(TypeStr),
		createDefaultConfig,
		processor.WithTraces(createTraceProcessor, component.StabilityLevelUndefined),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTraceProcessor(
	_ context.Context,
	cp processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {

	oCfg := cfg.(*Config)
	return newTraceProcessor(nextConsumer, oCfg.LoggingConfig)
}
