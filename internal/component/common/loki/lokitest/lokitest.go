// Package lokitest provides helpers for testing loki components.
package lokitest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

// Test runs fn as a subtest for each Loki pipeline, with an Env set up for
// that pipeline.
func Test(t *testing.T, fn func(t *testing.T, env Env)) {
	t.Helper()

	t.Run("pipeline v1", func(t *testing.T) {
		fn(t, Env{pipelineV2: false})
	})

	t.Run("pipeline v2", func(t *testing.T) {
		fn(t, Env{pipelineV2: true})
	})
}

// Env is the environment a test runs loki components in.
type Env struct {
	pipelineV2 bool
}

// Run builds and runs the component registered as name with args in env,
// and waits for it to be running.
// The returned stop function stops the component and waits for it to exit.
// It is called during test cleanup if the test did not call it.
func (env Env) Run(
	t *testing.T,
	name string,
	args component.Arguments,
	mods ...func(component.Options) component.Options,
) (*componenttest.Controller, func()) {

	t.Helper()

	ctrl, err := componenttest.NewControllerFromID(nil, name)
	require.NoError(t, err)

	var (
		ctx, cancel = context.WithCancel(t.Context())
		errCh       = make(chan error, 1)
	)

	go func() {
		errCh <- ctrl.Run(
			ctx,
			args,
			append([]func(component.Options) component.Options{env.opts}, mods...)...,
		)
	}()

	var (
		once    sync.Once
		stopErr error
	)

	stop := func() {
		t.Helper()
		once.Do(func() {
			cancel()
			select {
			case stopErr = <-errCh:
			case <-time.After(10 * time.Second):
				stopErr = errors.New("timed out waiting for the component to exit")
			}
		})
		require.NoError(t, stopErr)
	}
	t.Cleanup(stop)

	require.NoError(t, ctrl.WaitRunning(time.Minute))
	return ctrl, stop
}

func (env Env) opts(o component.Options) component.Options {
	svc := lokipipelinev2.New(lokipipelinev2.Options{PipelineV2: env.pipelineV2})

	getServiceData := o.GetServiceData
	o.GetServiceData = func(name string) (any, error) {
		if name == lokipipelinev2.ServiceName {
			return svc.Data(), nil
		}
		if getServiceData == nil {
			return nil, fmt.Errorf("service not found %s", name)
		}
		return getServiceData(name)
	}
	return o
}

// NewReceiver returns a Receiver for a component running in env to forward to.
func (env Env) NewReceiver(t *testing.T) *Receiver {
	if !env.pipelineV2 {
		handler := loki.NewCollectingHandler()
		t.Cleanup(handler.Stop)
		return &Receiver{recv: handler.Receiver(), entries: handler.Received}
	}

	consumer := loki.NewCollectingConsumer()
	return &Receiver{
		recv:     loki.NewLogsReceiverV2(consumer, "lokitest"),
		consumer: consumer,
		entries:  consumer.Entries,
	}
}

var (
	_ loki.Consumer     = (*Receiver)(nil)
	_ loki.LogsReceiver = (*Receiver)(nil)
)

// Receiver is a receiver for a component to forward to. It collects every
// entry it receives so tests can assert on them.
type Receiver struct {
	recv     loki.LogsReceiver
	consumer *loki.CollectingConsumer
	entries  func() []loki.Entry

	// consumed is the number of entries returned by Next so far.
	consumed int
}

func (r *Receiver) Chan() chan loki.Entry {
	return r.recv.Chan()
}

// Consume implements loki.Consumer. It returns an error with Loki pipeline V1.
func (r *Receiver) Consume(ctx context.Context, batch loki.Batch) error {
	if r.consumer == nil {
		return errors.New("lokitest: receiver does not support Loki pipeline V2")
	}
	return r.consumer.Consume(ctx, batch)
}

// Next waits until n entries arrive after the ones previous calls returned.
func (r *Receiver) Next(t *testing.T, n int) []loki.Entry {
	t.Helper()

	end := r.consumed + n
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.GreaterOrEqual(c, len(r.entries()), end)
	}, 5*time.Second, 10*time.Millisecond, "waiting for %d entries", n)

	entries := r.entries()[r.consumed:end]
	r.consumed = end
	return entries
}
