package foreach

import (
	"context"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "foreach",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
}

// SetToDefault implements syntax.Defaulter.
func (arg *Arguments) SetToDefault() {
}

// Validate implements syntax.Validator.
func (arg *Arguments) Validate() error {
	return nil
}

type Component struct {
}

var (
	_ component.Component = (*Component)(nil)
)

func New(o component.Options, args Arguments) (*Component, error) {
	return &Component{}, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	return nil
}
