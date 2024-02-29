package testcomponents

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.experimental",
		Stability: featuregate.StabilityExperimental,

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return &Experimental{log: opts.Logger}, nil
		},
	})
}

// Experimental is a test component that is marked as experimental. Used to verify stability level checking.
type Experimental struct {
	log log.Logger
}

func (e *Experimental) Run(ctx context.Context) error {
	e.log.Log("msg", "running experimental component")
	<-ctx.Done()
	return nil
}

func (e *Experimental) Update(args component.Arguments) error {
	e.log.Log("msg", "updating experimental component")
	return nil
}
