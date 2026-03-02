package testcomponents

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.string_receiver",
		Stability: featuregate.StabilityPublicPreview,
		Args:      StringReceiverConfig{},
		Exports:   StringReceiverExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewStringReceiverComp(opts, args.(StringReceiverConfig))
		},
	})
}

type StringReceiverConfig struct {
}

type StringReceiver interface {
	Receive(string)
}

type StringReceiverImpl struct {
	log func(string)
}

func (r StringReceiverImpl) Receive(s string) {
	r.log(s)
}

type StringReceiverExports struct {
	Receiver StringReceiver `alloy:"receiver,attr"`
}

type StringReceiverComponent struct {
	opts component.Options
	log  log.Logger

	mut      sync.Mutex
	recvStr  string
	receiver StringReceiver
}

// NewStringReceiver creates a new string_receiver component.
func NewStringReceiverComp(o component.Options, cfg StringReceiverConfig) (*StringReceiverComponent, error) {
	s := &StringReceiverComponent{opts: o, log: o.Logger}
	s.receiver = StringReceiverImpl{
		log: func(str string) {
			s.mut.Lock()
			defer s.mut.Unlock()
			s.recvStr += str + "\n"
		},
	}

	o.OnStateChange(StringReceiverExports{
		Receiver: s.receiver,
	})

	return s, nil
}

var (
	_ component.Component = (*StringReceiverComponent)(nil)
)

// Run implements Component.
func (s *StringReceiverComponent) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Return the receiver as debug info instead of export to avoid evaluation loop.
func (s *StringReceiverComponent) DebugInfo() any {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.recvStr
}

// Update implements Component.
func (s *StringReceiverComponent) Update(args component.Arguments) error {
	return nil
}
