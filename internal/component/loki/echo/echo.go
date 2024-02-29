package echo

import (
	"context"
	"sync"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/flow/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.echo",
		Stability: featuregate.StabilityBeta,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.echo
// component.
type Arguments struct{}

// Exports holds the values exported by the loki.echo component.
type Exports struct {
	Receiver loki.LogsReceiver `river:"receiver,attr"`
}

// DefaultArguments defines the default settings for log scraping.
var DefaultArguments = Arguments{}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

var (
	_ component.Component = (*Component)(nil)
)

// Component implements the loki.source.file component.
type Component struct {
	opts component.Options

	mut      sync.RWMutex
	args     Arguments
	receiver loki.LogsReceiver
}

// New creates a new loki.echo component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:     o,
		receiver: loki.NewLogsReceiver(),
	}

	// Call to Update() once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: c.receiver})

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			level.Info(c.opts.Logger).Log("receiver", c.opts.ID, "entry", entry.Line, "labels", entry.Labels.String())
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	return nil
}
