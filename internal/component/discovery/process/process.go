//go:build linux

package process

import (
	"context"
	"regexp"
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
	var cgroupIDRegexp *regexp.Regexp
	var err error
	if args.CgroupIDRegex != "" {
		if cgroupIDRegexp, err = regexp.Compile(args.CgroupIDRegex); err != nil {
			return nil, err
		}
	}

	c := &Component{
		l:              opts.Logger,
		onStateChange:  opts.OnStateChange,
		cgroupIDRegexp: cgroupIDRegexp,
		argsUpdates:    make(chan Arguments),
		args:           args,
	}

	return c, nil
}

type Component struct {
	l              log.Logger
	onStateChange  func(e component.Exports)
	processes      []discovery.Target
	cgroupIDRegexp *regexp.Regexp
	argsUpdates    chan Arguments
	args           Arguments
}

func (c *Component) Run(ctx context.Context) error {
	doDiscover := func() error {
		processes, err := discover(c.l, &c.args.DiscoverConfig, c.cgroupIDRegexp)
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

	// Compile updated regexp
	if a.CgroupIDRegex != "" {
		var err error
		if c.cgroupIDRegexp, err = regexp.Compile(a.CgroupIDRegex); err != nil {
			return err
		}
	}

	return nil
}

func (c *Component) changed() {
	c.onStateChange(discovery.Exports{
		Targets: join(c.processes, c.args.Join),
	})
}
