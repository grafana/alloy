package testcomponents

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"go.uber.org/atomic"
)

// testcomponents.summation_receiver sums up the values that it receives via the exported int receiver.
// The sum is exposed via the DebugInfo instead of the Exports to avoid triggering an update loop.
// (the components that are using the exported receiver would be updated every time the sum would be updated)
func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.summation_receiver",
		Stability: featuregate.StabilityPublicPreview,
		Args:      SummationReceiverConfig{},
		Exports:   SummationReceiverExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSummationReceiver(opts, args.(SummationReceiverConfig))
		},
	})
}

type SummationReceiverConfig struct {
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

type SummationReceiverExports struct {
	Receiver IntReceiver `alloy:"receiver,attr"`
}

type SummationReceiver struct {
	opts component.Options
	log  log.Logger

	sum      atomic.Int32
	receiver IntReceiver
}

// NewSummationReceiver creates a new summation component.
func NewSummationReceiver(o component.Options, cfg SummationReceiverConfig) (*SummationReceiver, error) {
	s := &SummationReceiver{opts: o, log: o.Logger}
	s.receiver = IntReceiverImpl{
		incrementSum: func(i int) {
			s.sum.Add(int32(i))
		},
	}

	o.OnStateChange(SummationReceiverExports{
		Receiver: s.receiver,
	})

	return s, nil
}

var (
	_ component.Component = (*SummationReceiver)(nil)
)

// Run implements Component.
func (s *SummationReceiver) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Return the sum as debug info instead of export to avoid evaluation loop.
func (s *SummationReceiver) DebugInfo() any {
	return int(s.sum.Load())
}

// Update implements Component.
func (s *SummationReceiver) Update(args component.Arguments) error {
	return nil
}
