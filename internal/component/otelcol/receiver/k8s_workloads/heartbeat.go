package k8s_workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const heartbeatInterval = time.Minute

type clusterHeartbeat struct {
	SnapshotID  string               `json:"snapshot_id"`
	ObservedAt  string               `json:"observed_at"`
	Complete    bool                 `json:"complete"`
	Deployments []deploymentSnapshot `json:"deployments"`
}

type deploymentSnapshot struct {
	Namespace          string             `json:"namespace"`
	Name               string             `json:"name"`
	UID                string             `json:"uid"`
	ResourceVersion    string             `json:"resource_version"`
	Generation         int64              `json:"generation"`
	ObservedGeneration int64              `json:"observed_generation"`
	Revision           int64              `json:"revision"`
	RolloutID          string             `json:"rollout_id"`
	Status             string             `json:"status"`
	Paused             bool               `json:"paused"`
	Deleting           bool               `json:"deleting"`
	DesiredReplicas    int32              `json:"desired_replicas"`
	Replicas           int32              `json:"replicas"`
	UpdatedReplicas    int32              `json:"updated_replicas"`
	AvailableReplicas  int32              `json:"available_replicas"`
	Containers         []rolloutContainer `json:"containers"`
}

func snapshotDeployment(clusterUID string, d *appsv1.Deployment) deploymentSnapshot {
	revision, _ := strconv.ParseInt(d.Annotations[revisionAnnotation], 10, 64)
	snapshot := deploymentSnapshot{Namespace: d.Namespace, Name: d.Name, UID: string(d.UID), ResourceVersion: d.ResourceVersion, Generation: d.Generation, ObservedGeneration: d.Status.ObservedGeneration, Revision: revision, Status: rolloutStatus(d), Paused: d.Spec.Paused, Deleting: d.DeletionTimestamp != nil, Replicas: d.Status.Replicas, UpdatedReplicas: d.Status.UpdatedReplicas, AvailableReplicas: d.Status.AvailableReplicas, Containers: []rolloutContainer{}}
	snapshot.DesiredReplicas = 1
	if d.Spec.Replicas != nil {
		snapshot.DesiredReplicas = *d.Spec.Replicas
	}
	if revision > 0 {
		snapshot.RolloutID = stableID(clusterUID, string(d.UID), strconv.FormatInt(revision, 10))
	}
	for _, container := range d.Spec.Template.Spec.Containers {
		snapshot.Containers = append(snapshot.Containers, rolloutContainer{Name: container.Name, Image: container.Image})
	}
	for _, container := range d.Spec.Template.Spec.InitContainers {
		snapshot.Containers = append(snapshot.Containers, rolloutContainer{Name: container.Name, Init: true, Image: container.Image})
	}
	return snapshot
}

// A successful fresh LIST proves API access and produces a consistent snapshot
// across all pages. Never claim completeness from a potentially stale watch cache.
func (c *controller) heartbeat(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, heartbeatInterval/2)
	defer cancel()
	now := c.now()
	observedAt := now.UTC().Format(time.RFC3339Nano)
	id := stableID(c.opts.clusterUID, "heartbeat", observedAt)
	payload := clusterHeartbeat{SnapshotID: id, ObservedAt: observedAt, Complete: true, Deployments: []deploymentSnapshot{}}
	options := metav1.ListOptions{Limit: 500}
	for {
		page, err := c.opts.client.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, options)
		if err != nil {
			return fmt.Errorf("list Deployments for heartbeat: %w", err)
		}
		for i := range page.Items {
			payload.Deployments = append(payload.Deployments, snapshotDeployment(c.opts.clusterUID, &page.Items[i]))
		}
		options.Continue = page.Continue
		if options.Continue == "" {
			break
		}
	}
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
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		if err := c.heartbeat(ctx); err != nil && ctx.Err() == nil {
			c.opts.logger.Error("Unable to deliver cluster heartbeat; waiting for next snapshot", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
