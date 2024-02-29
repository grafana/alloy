package server

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/signals"
	"go.uber.org/atomic"
)

var signalContexts atomic.Int64

// SignalContext wraps a ctx which will be canceled if an interrupt is
// received.
//
// It is invalid to have two simultaneous SignalContexts per binary.
func SignalContext(ctx context.Context, l log.Logger) (context.Context, context.CancelFunc) {
	if !signalContexts.CompareAndSwap(0, 1) {
		panic("bug: multiple SignalContexts found")
	}

	if l == nil {
		l = log.NewNopLogger()
	}

	ctx, cancel := context.WithCancel(ctx)

	handler := signals.NewHandler(l)
	go func() {
		handler.Loop()
		signalContexts.Store(0)
		cancel()
	}()
	go func() {
		<-ctx.Done()
		handler.Stop()
		signalContexts.Store(0)
	}()

	return ctx, cancel
}
