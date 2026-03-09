package file_match

import (
	"context"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util/glob"
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

	mut            sync.Mutex
	args           Arguments
	watches        []watch
	watchDog       *time.Ticker
	targetsChanged chan struct{}
	globber        glob.Globber
}

// New creates a new local.file_match component.
func New(o component.Options, args Arguments) (*Component, error) {
	return newComponent(o, args, glob.NewGlobber())
}

func newComponent(o component.Options, args Arguments, globber glob.Globber) (*Component, error) {
	c := &Component{
		opts:     o,
		mut:      sync.Mutex{},
		args:     args,
		watches:  make([]watch, 0),
		watchDog: time.NewTicker(args.SyncPeriod),
		// Buffered channel to avoid blocking
		targetsChanged: make(chan struct{}, 1),
		globber:        globber,
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

	newArgs := args.(Arguments)

	// Check to see if our ticker timer needs to be reset.
	if newArgs.SyncPeriod != c.args.SyncPeriod {
		c.watchDog.Reset(newArgs.SyncPeriod)
	}

	c.args = newArgs

	// Rebuild watches
	c.watches = c.watches[:0]
	for _, v := range c.args.PathTargets {
		c.watches = append(c.watches, watch{
			target:          v,
			log:             c.opts.Logger,
			ignoreOlderThan: c.args.IgnoreOlderThan,
			globber:         c.globber,
		})
	}

	// Always trigger immediate check when Update is called
	select {
	case c.targetsChanged <- struct{}{}:
	default:
	}

	return nil
}

// Run satisfies the component interface.
func (c *Component) Run(ctx context.Context) error {
	defer c.watchDog.Stop()
	for {
		select {
		case <-c.targetsChanged:
			// When we get a signal that we have new targets we will get all watched files and
			// reset the timer.
			c.mut.Lock()
			c.watchDog.Reset(c.args.SyncPeriod)
			targets := c.getWatchedFiles()
			c.mut.Unlock()
			c.opts.OnStateChange(discovery.Exports{Targets: targets})
		case <-c.watchDog.C:
			// If we have not received a signal that we have new targets watch job will periodically
			// get all files that we should watch.
			c.mut.Lock()
			targets := c.getWatchedFiles()
			c.mut.Unlock()
			c.opts.OnStateChange(discovery.Exports{Targets: targets})
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
