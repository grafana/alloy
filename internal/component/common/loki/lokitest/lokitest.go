// Package lokitest provides helpers for testing loki components with both
// Loki pipeline V1 and Loki pipeline V2.
package lokitest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

// RunWithPipelines runs fn as a subtest once with Loki pipeline V1 and once
// with Loki pipeline V2.
func RunWithPipelines(t *testing.T, fn func(t *testing.T, p Pipeline)) {
	t.Helper()

	t.Run("pipeline v1", func(t *testing.T) {
		fn(t, Pipeline{pipelineV2: false})
	})

	t.Run("pipeline v2", func(t *testing.T) {
		fn(t, Pipeline{pipelineV2: true})
	})
}

type Pipeline struct {
	pipelineV2 bool
}

// Opts enables p for a component built with o. It can be passed to
// componenttest.Controller.Run as an options modifier.
func (p Pipeline) Opts(o component.Options) component.Options {
	svc := lokipipelinev2.New(lokipipelinev2.Options{PipelineV2: p.pipelineV2})
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

// Send sends entries to recv with p.
func (p Pipeline) Send(t *testing.T, recv loki.LogsReceiver, entries ...loki.Entry) {
	t.Helper()

	if !p.pipelineV2 {
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

// NewReceiver returns a Receiver that is forwarded to with p.
func (p Pipeline) NewReceiver(t *testing.T) *Receiver {
	if !p.pipelineV2 {
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

// NoNext requires that no entries arrive after the ones previous calls to
// Next returned, for d.
func (r *Receiver) NoNext(t *testing.T, d time.Duration) {
	t.Helper()

	require.Never(t, func() bool {
		return len(r.entries()) > r.consumed
	}, d, 10*time.Millisecond, "expected no more entries")
}
