package count

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/connector"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

// Default metric names and descriptions follow OpenTelemetry Collector conventions.
const (
	defaultMetricNameSpans = "trace.span.count"
	defaultMetricDescSpans = "The number of spans observed."

	defaultMetricNameSpanEvents = "trace.span.event.count"
	defaultMetricDescSpanEvents = "The number of span events observed."

	defaultMetricNameMetrics = "metric.count"
	defaultMetricDescMetrics = "The number of metrics observed."

	defaultMetricNameDataPoints = "metric.datapoint.count"
	defaultMetricDescDataPoints = "The number of data points observed."

	defaultMetricNameLogs = "log.record.count"
	defaultMetricDescLogs = "The number of log records observed."
)

const (
	kindSpans      = "spans"
	kindSpanEvents = "spanevents"
	kindMetrics    = "metrics"
	kindDataPoints = "datapoints"
	kindLogs       = "logs"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.count",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			f := countconnector.NewFactory()
			return connector.New(opts, f, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.connector.count component.
type Arguments struct {
	// Config match the configuration structure of the otel count connector.
	Spans      []MetricInfo `alloy:"spans,block,optional"`
	SpanEvents []MetricInfo `alloy:"spanevents,block,optional"`
	Metrics    []MetricInfo `alloy:"metrics,block,optional"`
	DataPoints []MetricInfo `alloy:"datapoints,block,optional"`
	Logs       []MetricInfo `alloy:"logs,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type MetricInfo struct {
	Name        string            `alloy:"name,attr,optional"`
	Description string            `alloy:"description,attr,optional"`
	Conditions  []string          `alloy:"conditions,attr,optional"`
	Attributes  []AttributeConfig `alloy:"attributes,block,optional"`
}

type AttributeConfig struct {
	Key          string `alloy:"key,attr"`
	DefaultValue any    `alloy:"default_value,attr,optional"`
}

func (args *Arguments) SetToDefault() {
	// Do not initialize slices here - let them be empty by default.
	// Default metrics are created in Convert() if no custom metrics are defined.
	args.DebugMetrics.SetToDefault()
}

func NewMetricInfo(kind string) MetricInfo {
	var name, desc string

	switch kind {
	case kindSpans:
		name = defaultMetricNameSpans
		desc = defaultMetricDescSpans
	case kindSpanEvents:
		name = defaultMetricNameSpanEvents
		desc = defaultMetricDescSpanEvents
	case kindMetrics:
		name = defaultMetricNameMetrics
		desc = defaultMetricDescMetrics
	case kindDataPoints:
		name = defaultMetricNameDataPoints
		desc = defaultMetricDescDataPoints
	case kindLogs:
		name = defaultMetricNameLogs
		desc = defaultMetricDescLogs
	}

	return MetricInfo{
		Name:        name,
		Description: desc,
	}
}

func (args *Arguments) Validate() error {
	return nil
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	// If no custom metrics are defined for a signal type, create default metrics
	spans := make(map[string]countconnector.MetricInfo, max(len(args.Spans), 1))
	if len(args.Spans) == 0 {
		info := NewMetricInfo(kindSpans)
		spans[info.Name] = info.Convert()
	} else {
		for _, info := range args.Spans {
			spans[info.Name] = info.Convert()
		}
	}

	spanEvents := make(map[string]countconnector.MetricInfo, max(len(args.SpanEvents), 1))
	if len(args.SpanEvents) == 0 {
		info := NewMetricInfo(kindSpanEvents)
		spanEvents[info.Name] = info.Convert()
	} else {
		for _, info := range args.SpanEvents {
			spanEvents[info.Name] = info.Convert()
		}
	}

	metrics := make(map[string]countconnector.MetricInfo, max(len(args.Metrics), 1))
	if len(args.Metrics) == 0 {
		info := NewMetricInfo(kindMetrics)
		metrics[info.Name] = info.Convert()
	} else {
		for _, info := range args.Metrics {
			metrics[info.Name] = info.Convert()
		}
	}

	dataPoints := make(map[string]countconnector.MetricInfo, max(len(args.DataPoints), 1))
	if len(args.DataPoints) == 0 {
		info := NewMetricInfo(kindDataPoints)
		dataPoints[info.Name] = info.Convert()
	} else {
		for _, info := range args.DataPoints {
			dataPoints[info.Name] = info.Convert()
		}
	}

	logs := make(map[string]countconnector.MetricInfo, max(len(args.Logs), 1))
	if len(args.Logs) == 0 {
		info := NewMetricInfo(kindLogs)
		logs[info.Name] = info.Convert()
	} else {
		for _, info := range args.Logs {
			logs[info.Name] = info.Convert()
		}
	}

	conf := &countconnector.Config{
		Spans:      spans,
		SpanEvents: spanEvents,
		Metrics:    metrics,
		DataPoints: dataPoints,
		Logs:       logs,
	}

	return conf, nil
}

func (mi *MetricInfo) Convert() countconnector.MetricInfo {
	a := make([]countconnector.AttributeConfig, len(mi.Attributes))
	for i, attr := range mi.Attributes {
		a[i] = attr.Convert()
	}
	return countconnector.MetricInfo{
		Description: mi.Description,
		Conditions:  mi.Conditions,
		Attributes:  a,
	}
}

func (a *AttributeConfig) Convert() countconnector.AttributeConfig {
	return countconnector.AttributeConfig{
		Key:          a.Key,
		DefaultValue: a.DefaultValue,
	}
}

// Extensions implements connector.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements connector.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements connector.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// ConnectorType() int implements connector.Arguments.
func (Arguments) ConnectorType() int {
	return connector.ConnectorLogsToMetrics | connector.ConnectorTracesToMetrics | connector.ConnectorMetricsToMetrics
}

// DebugMetricsConfig implements connector.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
