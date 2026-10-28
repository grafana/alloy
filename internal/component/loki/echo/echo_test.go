package echo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

func TestEcho(t *testing.T) {
	labels := model.LabelSet{"job": "test"}
	entry := push.Entry{Timestamp: time.Unix(0, 0), Line: "line"}

	t.Run("exports channel receiver when pipeline v2 is disabled", func(t *testing.T) {
		c, exports := newTestComponent(t, false)

		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error, 1)
		go func() { errCh <- c.Run(ctx) }()

		select {
		case exports.Receiver.Chan() <- loki.NewEntry(labels, entry):
		case <-time.After(5 * time.Second):
			require.FailNow(t, "entry was not received")
		}

		cancel()
		require.NoError(t, <-errCh)
	})

	t.Run("exports consumer receiver when pipeline v2 is enabled", func(t *testing.T) {
		_, exports := newTestComponent(t, true)

		consumer, ok := exports.Receiver.(loki.Consumer)
		require.True(t, ok)

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(labels, entry))
		require.NoError(t, consumer.Consume(t.Context(), batch))
	})

	t.Run("consume returns error after run returns when pipeline v2 is enabled", func(t *testing.T) {
		c, exports := newTestComponent(t, true)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, c.Run(ctx))

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(labels, entry))
		require.ErrorIs(t, exports.Receiver.(loki.Consumer).Consume(t.Context(), batch), loki.ErrConsumerStopped)
	})
}

func newTestComponent(t *testing.T, pipelineV2 bool) (*Component, Exports) {
	var exports Exports
	c, err := New(component.Options{
		ID:            "loki.echo.test",
		Logger:        logging.NewSlogNop(),
		OnStateChange: func(e component.Exports) { exports = e.(Exports) },
		GetServiceData: func(name string) (any, error) {
			if name == lokipipelinev2.ServiceName {
				return lokipipelinev2.New(lokipipelinev2.Options{PipelineV2: pipelineV2}), nil
			}
			return nil, fmt.Errorf("no service named %s defined", name)
		},
	}, Arguments{})
	require.NoError(t, err)
	return c, exports
}
