// Package discovery provides an otelcol.processor.discovery component.
package discovery

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingconsumer"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/livedebugging"
	promsdconsumer "github.com/grafana/alloy/internal/static/traces/promsdprocessor/consumer"
	"github.com/grafana/alloy/syntax"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.discovery",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.discovery component.
type Arguments struct {
	Targets         []discovery.Target `alloy:"targets,attr"`
	OperationType   string             `alloy:"operation_type,attr,optional"`
	PodAssociations []string           `alloy:"pod_associations,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var (
	_ syntax.Defaulter = (*Arguments)(nil)
	_ syntax.Validator = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		OperationType: promsdconsumer.OperationTypeUpsert,
		PodAssociations: []string{
			promsdconsumer.PodAssociationIPLabel,
			promsdconsumer.PodAssociationOTelIPLabel,
			promsdconsumer.PodAssociationk8sIPLabel,
			promsdconsumer.PodAssociationHostnameLabel,
			promsdconsumer.PodAssociationConnectionIP,
		},
	}
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	err := promsdconsumer.ValidateOperationType(args.OperationType)
	if err != nil {
		return err
	}

	err = promsdconsumer.ValidatePodAssociations(args.PodAssociations)
	if err != nil {
		return err
	}

	return nil
}

// Component is the otelcol.exporter.discovery component.
type Component struct {
	consumer              *promsdconsumer.Consumer
	logger                log.Logger
	liveDebuggingConsumer *livedebuggingconsumer.Consumer
	debugDataPublisher    livedebugging.DebugDataPublisher

	opts component.Options
	args Arguments
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new otelcol.exporter.discovery component.
func New(o component.Options, c Arguments) (*Component, error) {
	if c.Output.Logs != nil || c.Output.Metrics != nil {
		level.Warn(o.Logger).Log("msg", "non-trace output detected; this component only works for traces")
	}

	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	liveDebuggingConsumer := livedebuggingconsumer.New(debugDataPublisher.(livedebugging.DebugDataPublisher), o.ID)

	traces := c.Output.Traces
	if debugDataPublisher.(livedebugging.DebugDataPublisher).IsActive(livedebugging.ComponentID(o.ID)) {
		traces = append(traces, liveDebuggingConsumer)
	}

	consumerOpts := promsdconsumer.Options{
		// Don't bother setting up labels - this will be done by the Update() function.
		HostLabels:      map[string]discovery.Target{},
		OperationType:   c.OperationType,
		PodAssociations: c.PodAssociations,
		NextConsumer:    fanoutconsumer.Traces(traces),
	}
	consumer, err := promsdconsumer.NewConsumer(consumerOpts, o.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create a traces consumer due to error: %w", err)
	}

	res := &Component{
		consumer:              consumer,
		logger:                o.Logger,
		opts:                  o,
		debugDataPublisher:    debugDataPublisher.(livedebugging.DebugDataPublisher),
		liveDebuggingConsumer: liveDebuggingConsumer,
	}

	if err := res.Update(c); err != nil {
		return nil, err
	}

	// Export the consumer.
	// This will remain the same throughout the component's lifetime,
	// so we do this during component construction.
	export := lazyconsumer.New(context.Background())
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
	c.args = newConfig.(Arguments)

	hostLabels := make(map[string]discovery.Target)

	for _, labels := range c.args.Targets {
		host, err := promsdconsumer.GetHostFromLabels(labels)
		if err != nil {
			level.Warn(c.logger).Log("msg", "ignoring target, unable to find address", "err", err)
			continue
		}

		hostLabels[host] = promsdconsumer.NewTargetsWithNonInternalLabels(labels)
	}
	traces := c.args.Output.Traces
	if c.debugDataPublisher.IsActive(livedebugging.ComponentID(c.opts.ID)) {
		traces = append(traces, c.liveDebuggingConsumer)
	}

	err := c.consumer.UpdateOptions(promsdconsumer.Options{
		HostLabels:      hostLabels,
		OperationType:   c.args.OperationType,
		PodAssociations: c.args.PodAssociations,
		NextConsumer:    fanoutconsumer.Traces(traces),
	})

	if err != nil {
		return fmt.Errorf("failed to update consumer options due to error: %w", err)
	}

	return nil
}

func (c *Component) LiveDebugging(_ int) {
	c.Update(c.args)
}
