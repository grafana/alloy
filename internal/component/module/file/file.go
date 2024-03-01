package file

import (
	"context"
	"sync"

	"go.uber.org/atomic"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/local/file"
	"github.com/grafana/agent/internal/component/module"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "module.file",
		Stability: featuregate.StabilityBeta,
		Args:      Arguments{},
		Exports:   module.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the module.file component.
type Arguments struct {
	LocalFileArguments file.Arguments `river:",squash"`

	// Arguments to pass into the module.
	Arguments map[string]any `river:"arguments,block,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	a.LocalFileArguments = file.DefaultArguments
}

// Component implements the module.file component.
type Component struct {
	opts component.Options
	mod  *module.ModuleComponent

	mut     sync.RWMutex
	args    Arguments
	content rivertypes.OptionalSecret

	managedLocalFile *file.Component
	inUpdate         atomic.Bool
	isCreated        atomic.Bool
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new module.file component.
func New(o component.Options, args Arguments) (*Component, error) {
	m, err := module.NewModuleComponent(o)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts: o,
		mod:  m,
		args: args,
	}
	defer c.isCreated.Store(true)

	c.managedLocalFile, err = c.newManagedLocalComponent(o)
	if err != nil {
		return nil, err
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// NewManagedLocalComponent creates the new local.file managed component.
func (c *Component) newManagedLocalComponent(o component.Options) (*file.Component, error) {
	localFileOpts := o
	localFileOpts.OnStateChange = func(e component.Exports) {
		c.setContent(e.(file.Exports).Content)

		if !c.inUpdate.Load() && c.isCreated.Load() {
			// Any errors found here are reported via component health
			_ = c.mod.LoadFlowSource(c.getArgs().Arguments, c.getContent().Value)
		}
	}

	return file.New(localFileOpts, c.getArgs().LocalFileArguments)
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		err := c.managedLocalFile.Run(ctx)
		if err != nil {
			ch <- err
		}
	}()

	go c.mod.RunFlowController(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-ch:
			return err
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.inUpdate.Store(true)
	defer c.inUpdate.Store(false)

	newArgs := args.(Arguments)
	c.setArgs(newArgs)

	err := c.managedLocalFile.Update(newArgs.LocalFileArguments)
	if err != nil {
		return err
	}

	// Force a content load here and bubble up any error. This will catch problems
	// on initial load.
	return c.mod.LoadFlowSource(newArgs.Arguments, c.getContent().Value)
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	leastHealthy := component.LeastHealthy(
		c.managedLocalFile.CurrentHealth(),
		c.mod.CurrentHealth(),
	)

	// if both components are healthy - return c.mod's health, so we can have a stable Health.Message.
	if leastHealthy.Health == component.HealthTypeHealthy {
		return c.mod.CurrentHealth()
	}
	return leastHealthy
}

// getArgs is a goroutine safe way to get args
func (c *Component) getArgs() Arguments {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.args
}

// setArgs is a goroutine safe way to set args
func (c *Component) setArgs(args Arguments) {
	c.mut.Lock()
	c.args = args
	c.mut.Unlock()
}

// getContent is a goroutine safe way to get content
func (c *Component) getContent() rivertypes.OptionalSecret {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.content
}

// setContent is a goroutine safe way to set content
func (c *Component) setContent(content rivertypes.OptionalSecret) {
	c.mut.Lock()
	c.content = content
	c.mut.Unlock()
}
