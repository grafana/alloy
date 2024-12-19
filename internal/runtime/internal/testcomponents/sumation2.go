package testcomponents

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.summation2",
		Stability: featuregate.StabilityPublicPreview,
		Args:      SummationConfig_2{},
		Exports:   SummationExports_2{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSummation_2(opts)
		},
	})
}

type IntReceiver interface {
	ReceiveInt(int)
}

type IntReceiverImpl struct {
	incrementSum func(int)
}

func (r IntReceiverImpl) ReceiveInt(i int) {
	r.incrementSum(i)
}

type SummationConfig_2 struct {
}

type SummationExports_2 struct {
	Receiver IntReceiver `alloy:"receiver,attr"`
}

type Summation_2 struct {
	opts component.Options
	log  log.Logger

	reg      prometheus.Registerer
	counter  prometheus.Counter
	receiver IntReceiver
}

// NewSummation creates a new summation component.
func NewSummation_2(o component.Options) (*Summation_2, error) {
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "testcomponents_summation2",
		Help: "Summation of all integers received",
	})

	recv := IntReceiverImpl{
		incrementSum: func(i int) {
			counter.Add(float64(i))
		},
	}

	t := &Summation_2{
		opts:     o,
		log:      o.Logger,
		receiver: recv,
		reg:      o.Registerer,
		counter:  counter,
	}

	o.OnStateChange(SummationExports_2{
		Receiver: t.receiver,
	})

	return t, nil
}

var (
	_ component.Component = (*Summation)(nil)
)

// Run implements Component.
func (t *Summation_2) Run(ctx context.Context) error {
	if err := t.reg.Register(t.counter); err != nil {
		return err
	}
	defer t.reg.Unregister(t.counter)

	<-ctx.Done()
	return nil
}

// Update implements Component.
func (t *Summation_2) Update(args component.Arguments) error {
	return nil
}
