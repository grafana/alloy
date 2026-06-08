package testcomponents

import (
	"context"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.experimental",
		Stability: featuregate.StabilityExperimental,

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return &Experimental{opts}, nil
		},
	})
}

// Experimental is a test component that is marked as experimental. Used to verify stability level checking.
type Experimental struct {
	opts component.Options
}

func (e *Experimental) Run(ctx context.Context) error {
	e.opts.SLogger.Info("running experimental component")
	<-ctx.Done()
	return nil
}

func (e *Experimental) Update(args component.Arguments) error {
	e.opts.SLogger.Info("updating experimental component")
	return nil
}
