//go:build (linux && !(amd64 || arm64)) || !linux

package java

import (
	"context"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/flow/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.java",
		Stability: featuregate.StabilityBeta,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			_ = level.Warn(opts.Logger).Log("msg", "the pyroscope.java component only works on linux for amd64 and arm64; enabling it otherwise will do nothing")
			return &javaComponent{}, nil
		},
	})
}

type javaComponent struct {
}

func (j *javaComponent) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		}
	}
}
func (j *javaComponent) Update(args component.Arguments) error {
	return nil
}
