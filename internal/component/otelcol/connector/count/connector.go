package logsmatch

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/connector"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

const (
	defaultMetricNameSpans      = "trace.span.count"
	defaultMetricDescSpans      = "The number of spans observed."
	defaultMetricNameSpanEvents = "trace.span.event.count"
	defaultMetricDescSpanEvents = "The number of span events observed."

	defaultMetricNameMetrics    = "metric.count"
	defaultMetricDescMetrics    = "The number of metrics observed."
	defaultMetricNameDataPoints = "metric.datapoint.count"
	defaultMetricDescDataPoints = "The number of data points observed."

	defaultMetricNameLogs = "log.record.count"
	defaultMetricDescLogs = "The number of log records observed."

	defaultMetricNameProfiles = "profile.count"
	defaultMetricDescProfiles = "The number of profiles observed."
)

type Arguments struct {
	Spans        *MetricsInfo                     `alloy:"spans,block,optional"`
	SpanEvents   *MetricsInfo                     `alloy:"span_events,block,optional"`
	Metrics      *MetricsInfo                     `alloy:"metrics,block,optional"`
	DataPoints   *MetricsInfo                     `alloy:"data_points,block,optional"`
	Logs         *MetricsInfo                     `alloy:"logs,block,optional"`
	Profiles     *MetricsInfo                     `alloy:"profiles,block,optional"`
	Output       *otelcol.ConsumerArguments       `alloy:"output,block"`
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type MetricsInfo struct {
	Counts []MetricInfo `alloy:"count,block,optional"`
}

type MetricInfo struct {
	Name        string            `alloy:"name,attr"`
	Description string            `alloy:"description,attr,optional"`
	Conditions  []string          `alloy:"conditions,attr,optional"`
	Attributes  []AttributeConfig `alloy:"attributes,attr,optional"`
}

type AttributeConfig struct {
	Key          string `alloy:"key,attr"`
	DefaultValue any    `alloy:"default_value,attr,optional"`
}

var (
	_ syntax.Defaulter    = (*Arguments)(nil)
	_ connector.Arguments = (*Arguments)(nil)
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.count",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := countconnector.NewFactory()
			return connector.New(opts, factory, args.(Arguments))
		},
	})
}

func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	config := &countconnector.Config{
		Spans: map[string]countconnector.MetricInfo{
			defaultMetricNameSpans: {
				Description: defaultMetricDescSpans,
			},
		},
		SpanEvents: map[string]countconnector.MetricInfo{
			defaultMetricNameSpanEvents: {
				Description: defaultMetricDescSpanEvents,
			},
		},
		Metrics: map[string]countconnector.MetricInfo{
			defaultMetricNameMetrics: {
				Description: defaultMetricDescMetrics,
			},
		},
		DataPoints: map[string]countconnector.MetricInfo{
			defaultMetricNameDataPoints: {
				Description: defaultMetricDescDataPoints,
			},
		},
		Logs: map[string]countconnector.MetricInfo{
			defaultMetricNameLogs: {
				Description: defaultMetricDescLogs,
			},
		},
		Profiles: map[string]countconnector.MetricInfo{
			defaultMetricNameProfiles: {
				Description: defaultMetricDescProfiles,
			},
		},
	}

	convertMetricInfoSlice := func(input []MetricInfo) map[string]countconnector.MetricInfo {
		if input == nil {
			return nil
		}

		output := make(map[string]countconnector.MetricInfo, len(input))

		convertAttributeConfigSlice := func(input []AttributeConfig) []countconnector.AttributeConfig {
			if input == nil {
				return nil
			}

			output := make([]countconnector.AttributeConfig, 0, len(input))

			for _, attr := range input {
				output = append(output, countconnector.AttributeConfig{
					Key:          attr.Key,
					DefaultValue: attr.DefaultValue,
				})
			}

			return output
		}

		for _, info := range input {
			output[info.Name] = countconnector.MetricInfo{
				Description: info.Description,
				Conditions:  info.Conditions,
				Attributes:  convertAttributeConfigSlice(info.Attributes),
			}
		}

		return output
	}

	if args.Spans != nil {
		config.Spans = convertMetricInfoSlice(args.Spans.Counts)
	}

	if args.SpanEvents != nil {
		config.SpanEvents = convertMetricInfoSlice(args.SpanEvents.Counts)
	}

	if args.Metrics != nil {
		config.Metrics = convertMetricInfoSlice(args.Metrics.Counts)
	}

	if args.DataPoints != nil {
		config.DataPoints = convertMetricInfoSlice(args.DataPoints.Counts)
	}

	if args.Logs != nil {
		config.Logs = convertMetricInfoSlice(args.Logs.Counts)
	}

	if args.Profiles != nil {
		config.Profiles = convertMetricInfoSlice(args.Profiles.Counts)
	}

	return config, nil
}

func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

func (args Arguments) ConnectorType() int {
	return connector.ConnectorLogsToMetrics
}

func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
