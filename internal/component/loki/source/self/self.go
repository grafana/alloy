package self

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.self",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.self
// component.
type Arguments struct {
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

// Component implements the loki.source.self component.
type Component struct {
	opts component.Options
	mut  sync.Mutex
}

// New creates a new loki.source.self component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts: o,
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	logging.UnRegisterSelfReceivers(c.opts.ID)
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	logging.RegisterSelfReceivers(c.opts.ID, newArgs.ForwardTo)
	return nil
}
