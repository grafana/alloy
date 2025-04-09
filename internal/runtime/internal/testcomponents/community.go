package testcomponents

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.community",
		Args:      struct{}{},
		Community: true,

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return &Community{log: opts.Logger}, nil
		},
	})
}

// Community is a test community component.
type Community struct {
	log log.Logger
}

func (e *Community) Run(ctx context.Context) error {
	e.log.Log("msg", "running community component")
	<-ctx.Done()
	return nil
}

func (e *Community) Update(args component.Arguments) error {
	e.log.Log("msg", "updating community component")
	return nil
}
