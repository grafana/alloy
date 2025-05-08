// Package spanlogs provides an otelcol.connector.spanlogs component.
package spanlogs

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/syntax"
	"go.opentelemetry.io/collector/pdata/plog"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.spanlogs",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

// Arguments configures the otelcol.connector.spanlogs component.
type Arguments struct {
	Spans             bool           `alloy:"spans,attr,optional"`
	Roots             bool           `alloy:"roots,attr,optional"`
	Processes         bool           `alloy:"processes,attr,optional"`
	Events            bool           `alloy:"events,attr,optional"`
	SpanAttributes    []string       `alloy:"span_attributes,attr,optional"`
	ProcessAttributes []string       `alloy:"process_attributes,attr,optional"`
	EventAttributes   []string       `alloy:"event_attributes,attr,optional"`
	Overrides         OverrideConfig `alloy:"overrides,block,optional"`
	Labels            []string       `alloy:"labels,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

type OverrideConfig struct {
	LogsTag     string `alloy:"logs_instance_tag,attr,optional"`
	ServiceKey  string `alloy:"service_key,attr,optional"`
	SpanNameKey string `alloy:"span_name_key,attr,optional"`
	StatusKey   string `alloy:"status_key,attr,optional"`
	DurationKey string `alloy:"duration_key,attr,optional"`
	TraceIDKey  string `alloy:"trace_id_key,attr,optional"`
}

var (
	_ syntax.Defaulter = (*Arguments)(nil)
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	Overrides: OverrideConfig{
		LogsTag:     "traces",
		ServiceKey:  "svc",
		SpanNameKey: "span",
		StatusKey:   "status",
		DurationKey: "dur",
		TraceIDKey:  "tid",
	},
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Component is the otelcol.exporter.spanlogs component.
type Component struct {
	consumer *consumer

	opts component.Options

	debugDataPublisher livedebugging.DebugDataPublisher

	args Arguments

	updateMut sync.Mutex
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new otelcol.exporter.spanlogs component.
func New(o component.Options, c Arguments) (*Component, error) {
	if c.Output.Traces != nil || c.Output.Metrics != nil {
		level.Warn(o.Logger).Log("msg", "non-log output detected; this component only works for log outputs and trace inputs")
	}

	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	nextLogs := fanoutconsumer.Logs(c.Output.Logs)
	consumer, err := NewConsumer(c, nextLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to create a traces consumer due to error: %w", err)
	}

	res := &Component{
		opts:               o,
		consumer:           consumer,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	if err := res.Update(c); err != nil {
		return nil, err
	}

	// Export the consumer.
	// This will remain the same throughout the component's lifetime,
	// so we do this during component construction.
	export := lazyconsumer.New(context.Background(), o.ID)
	export.SetConsumers(res.consumer, nil, nil)
	o.OnStateChange(otelcol.ConsumerExports{Input: export})

	return res, nil
}

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	for range ctx.Done() {
		return nil
	}
	return nil
}

// Update implements Component.
func (c *Component) Update(newConfig component.Arguments) error {
	c.updateMut.Lock()
	defer c.updateMut.Unlock()
	c.args = newConfig.(Arguments)

	nextLogs := c.args.Output.Logs

	fanout := fanoutconsumer.Logs(nextLogs)
	logsInterceptor := interceptconsumer.Logs(fanout,
		func(ctx context.Context, ld plog.Logs) error {
			livedebuggingpublisher.PublishLogsIfActive(c.debugDataPublisher, c.opts.ID, ld, otelcol.GetComponentMetadata(nextLogs))
			return fanout.ConsumeLogs(ctx, ld)
		},
	)

	err := c.consumer.UpdateOptions(c.args, logsInterceptor)
	if err != nil {
		return fmt.Errorf("failed to update traces consumer due to error: %w", err)
	}

	return nil
}

func (c *Component) LiveDebugging() {}
