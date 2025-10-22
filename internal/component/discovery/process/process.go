//go:build linux

package process

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
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
	debugDataPublisher, err := opts.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               opts,
		l:                  opts.Logger,
		onStateChange:      opts.OnStateChange,
		argsUpdates:        make(chan Arguments),
		args:               args,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	return c, nil
}

type Component struct {
	opts          component.Options
	l             log.Logger
	onStateChange func(e component.Exports)
	processes     []discovery.Target
	argsUpdates   chan Arguments
	args          Arguments

	debugDataPublisher livedebugging.DebugDataPublisher
}

var _ component.Component = (*Component)(nil)
var _ component.LiveDebugging = (*Component)(nil)

func (c *Component) Run(ctx context.Context) error {
	doDiscover := func() error {
		processes, err := discover(c.l, &c.args.DiscoverConfig)
		if err != nil {
			return err
		}
		c.processes = convertProcesses(processes)
		c.changed()

		componentID := livedebugging.ComponentID(c.opts.ID)
		c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
			componentID,
			livedebugging.Target,
			uint64(len(c.processes)),
			func() string { return fmt.Sprintf("%s", c.processes) },
		))

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

func (c *Component) LiveDebugging() {}
