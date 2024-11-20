package testcomponents

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.summation1",
		Stability: featuregate.StabilityPublicPreview,
		Args:      SummationConfig_Entry{},
		Exports:   SummationExports_Entry{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSummation_Entry(opts, args.(SummationConfig_Entry))
		},
	})
}

// Accepts a single integer input and forwards it to all the components listed in forward_to.
type SummationConfig_Entry struct {
	Input int `alloy:"input,attr"`
	//TODO: What should the type be?
	ForwardTo []IntReceiver `alloy:"forward_to,attr"`
}

type SummationExports_Entry struct {
}

type Summation_Entry struct {
	opts component.Options
	log  log.Logger
}

// NewSummation creates a new summation component.
func NewSummation_Entry(o component.Options, cfg SummationConfig_Entry) (*Summation_Entry, error) {
	t := &Summation_Entry{opts: o, log: o.Logger}
	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component = (*Summation_Entry)(nil)
)

// Run implements Component.
func (t *Summation_Entry) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (t *Summation_Entry) Update(args component.Arguments) error {
	c := args.(SummationConfig_Entry)

	for _, r := range c.ForwardTo {
		r.ReceiveInt(c.Input)
	}

	return nil
}
