package write

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "sigil.write",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Exports are the set of fields exposed by the sigil.write component.
type Exports struct {
	Receiver sigil.GenerationsReceiver `alloy:"receiver,attr"`
}

// Component is the sigil.write component.
type Component struct {
	logger        log.Logger
	onStateChange func(Exports)
	metrics       *metrics

	mu       sync.Mutex
	receiver *fanOutClient
}

// New creates a new sigil.write component.
func New(opts component.Options, args Arguments) (*Component, error) {
	m := newMetrics(opts.Registerer)
	receiver, err := newFanOutClient(opts.Logger, args, m)
	if err != nil {
		return nil, err
	}

	c := &Component{
		logger:        opts.Logger,
		onStateChange: func(e Exports) { opts.OnStateChange(e) },
		metrics:       m,
		receiver:      receiver,
	}

	c.onStateChange(Exports{Receiver: receiver})
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	level.Info(c.logger).Log("msg", "terminating sigil.write")
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	receiver, err := newFanOutClient(c.logger, newArgs, c.metrics)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.receiver != nil {
		c.receiver.closeIdleConnections()
	}
	c.receiver = receiver
	c.onStateChange(Exports{Receiver: receiver})
	return nil
}
