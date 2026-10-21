// Package lokitest provides helpers for testing loki components.
package lokitest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

// Test runs fn as a subtest for V1 and V2 Loki pipelines, with an Env set up for
// that pipeline.
func Test(t *testing.T, fn func(t *testing.T, env Env)) {
	t.Helper()

	TestV1(t, fn)
	TestV2(t, fn)
}

// TestV1 runs fn as a subtest with an Env set up for Loki pipeline v1.
func TestV1(t *testing.T, fn func(t *testing.T, env Env)) {
	t.Helper()

	t.Run("pipeline v1", func(t *testing.T) {
		fn(t, Env{pipelineV2: false})
	})
}

// TestV2 runs fn as a subtest with an Env set up for Loki pipeline v2.
func TestV2(t *testing.T, fn func(t *testing.T, env Env)) {
	t.Helper()

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
	t testing.TB,
	name string,
	args component.Arguments,
	mods ...func(component.Options) component.Options,
) (*componenttest.Controller, func()) {

	t.Helper()

	ctrl, err := componenttest.NewControllerFromID(nil, name)
	require.NoError(t, err)

	stop := componenttest.RunInBackground(t, func(ctx context.Context) error {
		return ctrl.Run(
			ctx,
			args,
			append([]func(component.Options) component.Options{env.opts}, mods...)...,
		)
	})

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

// Send sends entries to recv in env.
func (env Env) Send(t *testing.T, recv loki.LogsReceiver, entries ...loki.Entry) {
	t.Helper()

	if !env.pipelineV2 {
		for _, e := range entries {
			select {
			case recv.Chan() <- e:
			case <-time.After(5 * time.Second):
				require.FailNow(t, "timed out sending entry")
			}
		}
		return
	}

	c, ok := recv.(loki.Consumer)
	require.True(t, ok, "receiver does not support Loki pipeline V2")

	batch := loki.NewBatch()
	for _, e := range entries {
		batch.AddEntry(e.Labels, e.Created(), e.Entry)
	}
	require.NoError(t, c.Consume(t.Context(), batch))
}

// NewReceiver returns a Receiver for a component running in env to forward to.
// It is stopped when the test ends.
func (env Env) NewReceiver(t testing.TB) *Receiver {
	var r *Receiver
	if env.pipelineV2 {
		consumer := NewCollectingConsumer()
		r = &Receiver{
			recv:     loki.NewLogsReceiverV2(consumer, "lokitest"),
			consumer: consumer,
			entries:  consumer.Entries,
			stopFn:   consumer.Stop,
		}
	} else {
		recv := NewCollectingReceiver()
		r = &Receiver{recv: recv, entries: recv.Entries, stopFn: recv.Stop}
	}

	t.Cleanup(r.Stop)
	return r
}
