package testcomponents

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/logging/level"
	"go.uber.org/atomic"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.summation",
		Stability: featuregate.StabilityPublicPreview,
		Args:      SummationConfig{},
		Exports:   SummationExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSummation(opts, args.(SummationConfig))
		},
	})
}

type SummationConfig struct {
	Input int `alloy:"input,attr"`
}

type SummationExports struct {
	Sum       int `alloy:"sum,attr"`
	LastAdded int `alloy:"last_added,attr"`
}

type Summation struct {
	opts component.Options
	log  log.Logger
	sum  atomic.Int32
}

// NewSummation creates a new summation component.
func NewSummation(o component.Options, cfg SummationConfig) (*Summation, error) {
	t := &Summation{opts: o, log: o.Logger}
	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component = (*Summation)(nil)
)

// Run implements Component.
func (t *Summation) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (t *Summation) Update(args component.Arguments) error {
	c := args.(SummationConfig)
	newSum := int(t.sum.Add(int32(c.Input)))

	level.Info(t.log).Log("msg", "updated sum", "value", newSum, "input", c.Input)
	t.opts.OnStateChange(SummationExports{Sum: newSum, LastAdded: c.Input})
	return nil
}
