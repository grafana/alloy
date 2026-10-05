package k8s_workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	defaultHeartbeatInterval = 5 * time.Minute
	heartbeatMinRest         = time.Minute
	heartbeatTimeout         = 30 * time.Second
)

type clusterHeartbeat struct {
	SnapshotID string `json:"snapshot_id"`
	ObservedAt string `json:"observed_at"`
	Healthy    bool   `json:"healthy"`
}

// Heartbeats report collector liveness and cluster identity, never workload state.
func (c *controller) heartbeat(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, heartbeatTimeout)
	defer cancel()
	now := c.now()
	observedAt := now.UTC().Format(time.RFC3339Nano)
	id := stableID(c.opts.clusterUID, "heartbeat", observedAt)
	payload := clusterHeartbeat{SnapshotID: id, ObservedAt: observedAt, Healthy: true}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("k8s.cluster.uid", c.opts.clusterUID)
	if c.opts.clusterName != "" {
		rl.Resource().Attributes().PutStr("k8s.cluster.name", c.opts.clusterName)
	}
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.k8s_workloads")
	record := sl.LogRecords().AppendEmpty()
	record.SetEventName("grafana.sdlc.k8s.cluster.heartbeat")
	record.SetTimestamp(pcommon.NewTimestampFromTime(now))
	record.SetObservedTimestamp(record.Timestamp())
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	record.Attributes().PutStr("grafana.sdlc.event.id", id)
	record.Attributes().PutInt("grafana.sdlc.schema.version", 1)
	record.Body().SetStr(string(encoded))
	// Bound the full OTLP JSON record, including escaping of the JSON body.
	wire, err := (&plog.JSONMarshaler{}).MarshalLogs(logs)
	if err != nil {
		return err
	}
	if len(wire) > 2*1024*1024-1024 {
		return fmt.Errorf("cluster heartbeat exceeds 2 MiB limit: %d bytes", len(wire))
	}
	// Heartbeats are replaced with a fresh observation on the next tick, never
	// retried in the rollout queue. Downstream queues must retain observed_at.
	return c.opts.emit(ctx, func() eventBatch {
		copy := plog.NewLogs()
		logs.CopyTo(copy)
		return eventBatch{logs: copy, id: id}
	})
}

func (c *controller) heartbeats(ctx context.Context) {
	for ctx.Err() == nil {
		started := time.Now()
		if err := c.heartbeat(ctx); err != nil && ctx.Err() == nil {
			c.opts.logger.Error("Unable to deliver cluster heartbeat; waiting for next snapshot", "err", err)
		}
		// Schedule from this attempt's start, but always allow a full minute
		// after it finishes. A slow or failed attempt never creates catch-up work.
		delay := max(c.opts.heartbeatInterval-time.Since(started), heartbeatMinRest)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
