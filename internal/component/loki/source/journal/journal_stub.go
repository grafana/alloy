//go:build !linux || !cgo || !promtail_journal_enabled

package journal

import (
	"context"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.journal",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

var _ component.Component = (*Component)(nil)

// Component represents reading from a journal
type Component struct {
}

// New creates a new  component.
func New(o component.Options, args Arguments) (*Component, error) {
	level.Info(o.Logger).Log("msg", "loki.source.journal is not enabled, and must be ran on linux with journal support")
	c := &Component{}
	return c, nil
}

// Run starts the component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update updates the fields of the component.
func (c *Component) Update(args component.Arguments) error {
	return nil
}
