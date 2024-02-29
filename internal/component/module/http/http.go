package http

import (
	"context"
	"sync"

	"go.uber.org/atomic"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/module"
	remote_http "github.com/grafana/agent/internal/component/remote/http"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "module.http",
		Stability: featuregate.StabilityBeta,
		Args:      Arguments{},
		Exports:   module.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the module.http component.
type Arguments struct {
	RemoteHTTPArguments remote_http.Arguments `river:",squash"`

	Arguments map[string]any `river:"arguments,block,optional"`
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	args.RemoteHTTPArguments.SetToDefault()
}

// Component implements the module.http component.
type Component struct {
	opts component.Options
	mod  *module.ModuleComponent

	mut     sync.RWMutex
	args    Arguments
	content rivertypes.OptionalSecret

	managedRemoteHTTP *remote_http.Component
	inUpdate          atomic.Bool
	isCreated         atomic.Bool
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new module.http component.
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

	c.managedRemoteHTTP, err = c.newManagedLocalComponent(o)
	if err != nil {
		return nil, err
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// NewManagedLocalComponent creates the new remote.http managed component.
func (c *Component) newManagedLocalComponent(o component.Options) (*remote_http.Component, error) {
	remoteHttpOpts := o
	remoteHttpOpts.OnStateChange = func(e component.Exports) {
		c.setContent(e.(remote_http.Exports).Content)

		if !c.inUpdate.Load() && c.isCreated.Load() {
			// Any errors found here are reported via component health
			_ = c.mod.LoadFlowSource(c.getArgs().Arguments, c.getContent().Value)
		}
	}

	return remote_http.New(remoteHttpOpts, c.getArgs().RemoteHTTPArguments)
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		err := c.managedRemoteHTTP.Run(ctx)
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

	err := c.managedRemoteHTTP.Update(newArgs.RemoteHTTPArguments)
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
		c.managedRemoteHTTP.CurrentHealth(),
		c.mod.CurrentHealth(),
	)

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
