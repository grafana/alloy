//go:build linux

package process

import (
	"context"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.process",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		l:             opts.Logger,
		onStateChange: opts.OnStateChange,
		argsUpdates:   make(chan Arguments),
		args:          args,
	}

	return c, nil
}

type Component struct {
	l             log.Logger
	onStateChange func(e component.Exports)
	processes     []discovery.Target
	argsUpdates   chan Arguments
	args          Arguments
}

func (c *Component) Run(ctx context.Context) error {
	doDiscover := func() error {
		processes, err := discover(c.l, &c.args.DiscoverConfig)
		if err != nil {
			return err
		}
		c.processes = convertProcesses(processes)
		c.changed()

		return nil
	}
	if err := doDiscover(); err != nil {
		return err
	}

	t := time.NewTicker(c.args.RefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := doDiscover(); err != nil {
				return err
			}
			t.Reset(c.args.RefreshInterval)
		case a := <-c.argsUpdates:
			c.args = a
			c.changed()
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	a := args.(Arguments)
	c.argsUpdates <- a
	return nil
}

func (c *Component) changed() {
	c.onStateChange(discovery.Exports{
		Targets: join(c.processes, c.args.Join),
	})
}
