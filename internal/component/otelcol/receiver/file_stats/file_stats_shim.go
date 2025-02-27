//go:build !linux && !darwin && !windows

// Package file_stats provides an otelcol.receiver.file_stats component.
package file_stats

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.file_stats",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			msg := fmt.Sprintf("otelcol.receiver.file_stats is not supported on %s; this instance of the component will do nothing", runtime.GOOS)

			level.Warn(opts.Logger).Log("msg", msg)
			return fakeComponent{
				health: component.Health{
					Health:     component.HealthTypeUnhealthy,
					Message:    msg,
					UpdateTime: time.Now(),
				},
			}, nil
		},
	})
}

type fakeComponent struct{ health component.Health }

var _ component.HealthComponent = fakeComponent{}

func (fc fakeComponent) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (fc fakeComponent) Update(_ component.Arguments) error {
	return nil
}

func (fc fakeComponent) CurrentHealth() component.Health {
	return fc.health
}
