// Package loki provides an otelcol.exporter.loki component.
package loki

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/loki/internal/convert"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.loki",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

// Arguments configures the otelcol.exporter.loki component.
type Arguments struct {
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

// Component is the otelcol.exporter.loki component.
type Component struct {
	log  log.Logger
	opts component.Options

	converter *convert.Converter
}

var _ component.Component = (*Component)(nil)

// New creates a new otelcol.exporter.loki component.
func New(o component.Options, c Arguments) (*Component, error) {
	converter := convert.New(o.Logger, o.Registerer, c.ForwardTo)

	res := &Component{
		log:  o.Logger,
		opts: o,

		converter: converter,
	}
	if err := res.Update(c); err != nil {
		return nil, err
	}

	// Construct a consumer based on our converter and export it. This will
	// remain the same throughout the component's lifetime, so we do this
	// during component construction.
	export := lazyconsumer.New(context.Background(), o.ID)
	export.SetConsumers(nil, nil, converter)
	o.OnStateChange(otelcol.ConsumerExports{Input: export})

	return res, nil
}

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (c *Component) Update(newConfig component.Arguments) error {
	cfg := newConfig.(Arguments)
	c.converter.UpdateFanout(cfg.ForwardTo)
	return nil
}
