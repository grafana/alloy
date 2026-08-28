// Package targets is its own package to break the import cycle where runtime is importing
// testcomponents (instead of e.g. runtime_test importing testcomponents)
package targets

import (
	"context"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.targets",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      TargetsConfig{},
		Exports:   TargetsExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewTargets(opts, args.(TargetsConfig))
		},
	})
}

// TargetsConfig configures the testcomponents.targets component.
type TargetsConfig struct {
	Input []discovery.Target `alloy:"targets,attr"`
}

// TargetsExports describes exported fields for the
// testcomponents.targets component.
type TargetsExports struct {
	Output []discovery.Target `alloy:"output,attr,optional"`
}

// Targets implements the testcomponents.targets component, where it
// always emits its input as an output.
type Targets struct {
	opts component.Options
}

// NewTargets creates a new targets component.
func NewTargets(o component.Options, cfg TargetsConfig) (*Targets, error) {
	t := &Targets{opts: o}
	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component = (*Targets)(nil)
)

// Run implements Component.
func (t *Targets) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (t *Targets) Update(args component.Arguments) error {
	c := args.(TargetsConfig)
	t.opts.OnStateChange(TargetsExports{Output: c.Input})
	return nil
}
