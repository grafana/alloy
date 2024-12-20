package file_match

import (
	"context"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "local.file_match",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the local.file_match
// component.
type Arguments struct {
	PathTargets     []discovery.Target `alloy:"path_targets,attr"`
	SyncPeriod      time.Duration      `alloy:"sync_period,attr,optional"`
	IgnoreOlderThan time.Duration      `alloy:"ignore_older_than,attr,optional"`
}

var _ component.Component = (*Component)(nil)

// Component implements the local.file_match component.
type Component struct {
	opts component.Options

	mut      sync.RWMutex
	args     Arguments
	watches  []watch
	watchDog *time.Ticker
}

// New creates a new local.file_match component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:     o,
		mut:      sync.RWMutex{},
		args:     args,
		watches:  make([]watch, 0),
		watchDog: time.NewTicker(args.SyncPeriod),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

func getDefault() Arguments {
	return Arguments{SyncPeriod: 10 * time.Second}
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = getDefault()
}

// Update satisfies the component interface.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	// Check to see if our ticker timer needs to be reset.
	if args.(Arguments).SyncPeriod != c.args.SyncPeriod {
		c.watchDog.Reset(c.args.SyncPeriod)
	}
	c.args = args.(Arguments)
	c.watches = c.watches[:0]
	for _, v := range c.args.PathTargets {
		c.watches = append(c.watches, watch{
			target:          v,
			log:             c.opts.Logger,
			ignoreOlderThan: c.args.IgnoreOlderThan,
		})
	}

	return nil
}

// Run satisfies the component interface.
func (c *Component) Run(ctx context.Context) error {
	update := func() {
		c.mut.Lock()
		defer c.mut.Unlock()

		paths := c.getWatchedFiles()
		// The component node checks to see if exports have actually changed.
		c.opts.OnStateChange(discovery.Exports{Targets: paths})
	}
	// Trigger initial check
	update()
	defer c.watchDog.Stop()
	for {
		select {
		case <-c.watchDog.C:
			// This triggers a check for any new paths, along with pushing new targets.
			update()
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *Component) getWatchedFiles() []discovery.Target {
	paths := make([]discovery.Target, 0)
	// See if there is anything new we need to check.
	for _, w := range c.watches {
		newPaths, err := w.getPaths()
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error getting paths", "path", w.getPath(), "excluded", w.getExcludePath(), "err", err)
		}
		paths = append(paths, newPaths...)
	}
	return paths
}
