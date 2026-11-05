package rustgen

import (
	"context"
	"errors"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.rustgen",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.rustgen
// component.
type Arguments struct {
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

var _ component.Component = (*Component)(nil)

type Component struct {
	opts   component.Options
	fanout *loki.FanoutV2
}

// New creates a new loki.source.rustgen component.
func New(o component.Options, args Arguments) (*Component, error) {
	svc, err := o.GetServiceData(lokipipelinev2.ServiceName)
	if err != nil {
		return nil, err
	}

	if !svc.(lokipipelinev2.Data).PipelineV2Enabled() {
		return nil, errors.New("loki.source.rustgen requires Loki pipeline V2")
	}

	c := &Component{
		opts:   o,
		fanout: loki.NewFanoutV2(args.ForwardTo),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.fanout.UpdateChildren(newArgs.ForwardTo)
	return nil
}
