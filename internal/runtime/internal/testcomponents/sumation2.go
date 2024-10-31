package testcomponents

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"go.uber.org/atomic"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.summation2",
		Stability: featuregate.StabilityPublicPreview,
		Args:      SummationConfig_2{},
		Exports:   SummationExports_2{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSummation_2(opts, args.(SummationConfig_2))
		},
	})
}

type IntReceiver interface {
	ReceiveInt(int)
}

type IntReceiverImpl struct {
	sum atomic.Int32
}

func (r IntReceiverImpl) ReceiveInt(i int) {
	r.sum.Add(int32(i))
}

type SummationConfig_2 struct {
}

type SummationExports_2 struct {
	Receiver  IntReceiver `alloy:"receiver,attr"`
	Sum       int         `alloy:"sum,attr"`
	LastAdded int         `alloy:"last_added,attr"`
}

type Summation_2 struct {
	opts     component.Options
	log      log.Logger
	receiver IntReceiver
}

// NewSummation creates a new summation component.
func NewSummation_2(o component.Options, cfg SummationConfig_2) (*Summation_2, error) {
	recv := IntReceiverImpl{}
	o.OnStateChange(SummationExports_2{
		Receiver: recv,
	})

	t := &Summation_2{
		opts:     o,
		log:      o.Logger,
		receiver: recv,
	}

	return t, nil
}

var (
	_ component.Component = (*Summation)(nil)
)

// Run implements Component.
func (t *Summation_2) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (t *Summation_2) Update(args component.Arguments) error {
	return nil
}
