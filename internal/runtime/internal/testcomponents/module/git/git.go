// Package git implements the module.git component.
package git

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents/module"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/vcs"
)

func init() {
	component.Register(component.Registration{
		Name:      "module.git",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   module.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the module.git component.
type Arguments struct {
	Repository    string        `alloy:"repository,attr"`
	Revision      string        `alloy:"revision,attr,optional"`
	Path          string        `alloy:"path,attr"`
	PullFrequency time.Duration `alloy:"pull_frequency,attr,optional"`

	Arguments     map[string]any    `alloy:"arguments,block,optional"`
	GitAuthConfig vcs.GitAuthConfig `alloy:",squash"`
}

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	Revision:      "HEAD",
	PullFrequency: time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Component implements the module.git component.
type Component struct {
	opts component.Options
	log  log.Logger
	mod  *module.ModuleComponent

	mut      sync.RWMutex
	repo     *vcs.GitRepo
	repoOpts vcs.GitRepoOptions
	args     Arguments

	argsChanged chan struct{}

	healthMut sync.RWMutex
	health    component.Health
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new module.git component.
func New(o component.Options, args Arguments) (*Component, error) {
	m, err := module.NewModuleComponent(o)
	if err != nil {
		return nil, err
	}
	c := &Component{
		opts: o,
		log:  o.Logger,

		mod: m,

		argsChanged: make(chan struct{}, 1),
	}

	// Only acknowledge the error from Update if it's not a
	// vcs.UpdateFailedError; vcs.UpdateFailedError means that the Git repo
	// exists but we were just unable to update it.
	if err := c.Update(args); err != nil {
		if errors.As(err, &vcs.UpdateFailedError{}) {
			level.Error(c.log).Log("msg", "failed to update repository", "err", err)
		} else {
			return nil, err
		}
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.mod.RunAlloyController(ctx)

	var (
		ticker  *time.Ticker
		tickerC <-chan time.Time
	)

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-c.argsChanged:
			c.mut.Lock()
			{
				level.Info(c.log).Log("msg", "updating repository pull frequency", "new_frequency", c.args.PullFrequency)

				if c.args.PullFrequency > 0 {
					if ticker == nil {
						ticker = time.NewTicker(c.args.PullFrequency)
						tickerC = ticker.C
					} else {
						ticker.Reset(c.args.PullFrequency)
					}
				} else {
					if ticker != nil {
						ticker.Stop()
					}
					ticker = nil
					tickerC = nil
				}
			}
			c.mut.Unlock()

		case <-tickerC:
			level.Info(c.log).Log("msg", "updating repository", "new_frequency", c.args.PullFrequency)
			c.tickPollFile(ctx)
		}
	}
}

func (c *Component) tickPollFile(ctx context.Context) {
	c.mut.Lock()
	err := c.pollFile(ctx, c.args)
	c.mut.Unlock()

	c.updateHealth(err)
}

func (c *Component) updateHealth(err error) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()

	if err != nil {
		c.health = component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    err.Error(),
			UpdateTime: time.Now(),
		}
	} else {
		c.health = component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "module updated",
			UpdateTime: time.Now(),
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) (err error) {
	defer func() {
		c.updateHealth(err)
	}()

	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	// TODO(rfratto): store in a repo-specific directory so changing repositories
	// doesn't risk break the module loader if there's a SHA collision between
	// the two different repositories.
	repoPath := filepath.Join(c.opts.DataPath, "repo")

	repoOpts := vcs.GitRepoOptions{
		Repository: newArgs.Repository,
		Revision:   newArgs.Revision,
		Auth:       newArgs.GitAuthConfig,
	}

	// Create or update the repo field.
	// Failure to update repository makes the module loader temporarily use cached contents on disk
	if c.repo == nil || !equality.DeepEqual(repoOpts, c.repoOpts) {
		r, err := vcs.NewGitRepo(context.Background(), repoPath, repoOpts)
		if err != nil {
			if errors.As(err, &vcs.UpdateFailedError{}) {
				level.Error(c.log).Log("msg", "failed to update repository", "err", err)
				c.updateHealth(err)
			} else {
				return err
			}
		}
		c.repo = r
		c.repoOpts = repoOpts
	}

	if err := c.pollFile(context.Background(), newArgs); err != nil {
		return err
	}

	// Schedule an update for handling the changed arguments.
	select {
	case c.argsChanged <- struct{}{}:
	default:
	}

	c.args = newArgs
	return nil
}

// pollFile fetches the latest content from the repository and updates the
// controller. pollFile must only be called with c.mut held.
func (c *Component) pollFile(ctx context.Context, args Arguments) error {
	// Make sure our repo is up-to-date.
	if err := c.repo.Update(ctx); err != nil {
		return err
	}

	// Finally, configure our controller.
	bb, err := c.repo.ReadFile(args.Path)
	if err != nil {
		return err
	}

	return c.mod.LoadAlloySource(args.Arguments, string(bb))
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()

	return component.LeastHealthy(c.health, c.mod.CurrentHealth())
}

// DebugInfo implements component.DebugComponent.
func (c *Component) DebugInfo() any {
	type DebugInfo struct {
		SHA       string `alloy:"sha,attr"`
		RepoError string `alloy:"repo_error,attr,optional"`
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	rev, err := c.repo.CurrentRevision()
	if err != nil {
		return DebugInfo{RepoError: err.Error()}
	} else {
		return DebugInfo{SHA: rev}
	}
}
