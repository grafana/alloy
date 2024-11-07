package dbo11y

import (
	"context"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "dbo11y",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	SomeArg string `alloy:"somearg,attr,optional"`
}

type Component struct {
	log log.Logger
}

func New(opts component.Options, args Arguments) (*Component, error) {
	return &Component{
		log: opts.Logger,
	}, nil
}

func (c *Component) Run(ctx context.Context) error {
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	return nil
}
