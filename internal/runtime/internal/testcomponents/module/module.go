package module

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// ModuleComponent holds the common properties for module components.
type ModuleComponent struct {
	opts component.Options
	mod  component.Module

	mut           sync.RWMutex
	health        component.Health
	latestContent string
	latestArgs    map[string]any
}

// Exports holds values which are exported from the run module.
type Exports struct {
	// Exports exported from the running module.
	Exports map[string]any `alloy:"exports,block"`
}

// NewModuleComponent initializes a new ModuleComponent.
func NewModuleComponent(o component.Options) (*ModuleComponent, error) {
	c := &ModuleComponent{
		opts: o,
	}
	var err error
	c.mod, err = o.ModuleController.NewModule("", func(exports map[string]any) {
		c.opts.OnStateChange(Exports{Exports: exports})
	})
	return c, err
}

// LoadAlloySource loads the controller with the current component source.
// It will set the component health in addition to return the error so that the consumer can rely on either or both.
// If the content is the same as the last time it was successfully loaded, it will not be reloaded.
func (c *ModuleComponent) LoadAlloySource(args map[string]any, contentValue string) error {
	if equality.DeepEqual(args, c.getLatestArgs()) && contentValue == c.getLatestContent() {
		return nil
	}

	err := c.mod.LoadConfig([]byte(contentValue), args)
	if err != nil {
		c.setHealth(component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    fmt.Sprintf("failed to load module content: %s", err),
			UpdateTime: time.Now(),
		})

		return err
	}

	c.setLatestArgs(args)
	c.setLatestContent(contentValue)
	c.setHealth(component.Health{
		Health:     component.HealthTypeHealthy,
		Message:    "module content loaded",
		UpdateTime: time.Now(),
	})

	return nil
}

// RunAlloyController runs the controller that all module components start.
func (c *ModuleComponent) RunAlloyController(ctx context.Context) {
	err := c.mod.Run(ctx)
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "error running module", "id", c.opts.ID, "err", err)
	}
}

// CurrentHealth contains the implementation details for CurrentHealth in a module component.
func (c *ModuleComponent) CurrentHealth() component.Health {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.health
}

// SetHealth contains the implementation details for setHealth in a module component.
func (c *ModuleComponent) setHealth(h component.Health) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.health = h
}

func (c *ModuleComponent) setLatestContent(content string) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.latestContent = content
}

func (c *ModuleComponent) getLatestContent() string {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.latestContent
}

func (c *ModuleComponent) setLatestArgs(args map[string]any) {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.latestArgs = make(map[string]any)
	maps.Copy(c.latestArgs, args)
}

func (c *ModuleComponent) getLatestArgs() map[string]any {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.latestArgs
}
