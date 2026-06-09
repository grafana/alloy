package testcomponents

import (
	"context"

	"github.com/grafana/alloy/internal/component"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.community",
		Args:      struct{}{},
		Community: true,

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return &Community{opts: opts}, nil
		},
	})
}

// Community is a test community component.
type Community struct {
	opts component.Options
}

func (e *Community) Run(ctx context.Context) error {
	e.opts.Logger.Info("running community component")
	<-ctx.Done()
	return nil
}

func (e *Community) Update(args component.Arguments) error {
	e.opts.Logger.Info("updating community component")
	return nil
}
