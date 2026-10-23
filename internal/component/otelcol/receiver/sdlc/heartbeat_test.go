package sdlc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHeartbeatHealthOnly(t *testing.T) {
	c := testController(t)
	client := fake.NewSimpleClientset()
	c.opts.client = client
	c.opts.clusterName = "test"
	var logs plog.Logs
	c.opts.emit = func(_ context.Context, batch func() eventBatch) error { logs = batch().logs; return nil }
	require.NoError(t, c.heartbeat(t.Context()))
	require.Empty(t, client.Actions(), "heartbeat must not query workload inventory")
	resource := logs.ResourceLogs().At(0)
	name, ok := resource.Resource().Attributes().Get("k8s.cluster.name")
	require.True(t, ok)
	require.Equal(t, "test", name.Str())
	record := resource.ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "grafana.sdlc.k8s.cluster.heartbeat", record.EventName())
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(record.Body().Str()), &payload))
	require.Equal(t, true, payload["healthy"])
	require.NotContains(t, payload, "deployments")
	require.Equal(t, stableID("cluster-1", "heartbeat", payload["observed_at"].(string)), payload["snapshot_id"])
	require.Zero(t, c.queue.Len())
}

func TestHeartbeatDeliveryFailureDoesNotQueueRetry(t *testing.T) {
	c := testController(t)
	c.opts.client = fake.NewSimpleClientset()
	c.opts.emit = func(_ context.Context, _ func() eventBatch) error { return errors.New("unavailable") }
	require.Error(t, c.heartbeat(t.Context()))
	require.Zero(t, c.queue.Len())
}

func TestHeartbeatSchedule(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		interval, duration, spacing time.Duration
		fail                        bool
	}{
		{"default", 5 * time.Minute, 0, 5 * time.Minute, false},
		{"custom", 10 * time.Minute, 0, 10 * time.Minute, false},
		{"minimum rest", time.Minute, 20 * time.Second, 80 * time.Second, false},
		{"overrun", 5 * time.Minute, 6 * time.Minute, 7 * time.Minute, false},
		{"failure", time.Minute, 20 * time.Second, 80 * time.Second, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := testController(t)
				c.opts.client = fake.NewSimpleClientset()
				c.opts.heartbeatInterval = tc.interval
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var starts []time.Time
				start := time.Now()
				c.opts.emit = func(context.Context, func() eventBatch) error {
					starts = append(starts, time.Now())
					if len(starts) == 3 {
						cancel()
					}
					// Model slow downstream work, including a consumer which does
					// not immediately return when the attempt deadline expires.
					time.Sleep(tc.duration)
					if tc.fail {
						return errors.New("unavailable")
					}
					return nil
				}
				c.heartbeats(ctx)
				require.Equal(t, []time.Time{start, start.Add(tc.spacing), start.Add(2 * tc.spacing)}, starts)
			})
		})
	}
}

func TestHeartbeatWaitCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := testController(t)
		c.opts.client = fake.NewSimpleClientset()
		c.opts.heartbeatInterval = 5 * time.Minute
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		calls := 0
		c.opts.emit = func(context.Context, func() eventBatch) error { calls++; return nil }
		done := make(chan struct{})
		go func() { c.heartbeats(ctx); close(done) }()
		synctest.Wait()
		require.Equal(t, 1, calls)
		cancel()
		<-done
		require.Equal(t, 1, calls)
	})
}
