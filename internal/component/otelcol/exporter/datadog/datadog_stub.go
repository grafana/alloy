//go:build freebsd || openbsd

package datadog

import (
	"context"

	"github.com/grafana/alloy/internal/component"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.datadog",
		Community: true,
		Args:      Argument{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			opts.SLogger.Warn("otelcol.exporter.datadog is unsupported on freebsd")
			return &FakeComponent{}, nil
		},
	})
}

var (
	_ component.Component = (*FakeComponent)(nil)
)

// FakeComponent implements the otelcol.exporter.datadog component for freebsd environments.
type FakeComponent struct{}

type Argument struct{}

func (f *FakeComponent) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (f *FakeComponent) Update(_ component.Arguments) error {
	return nil
}
