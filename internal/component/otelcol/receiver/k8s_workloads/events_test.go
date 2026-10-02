package k8s_workloads

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func deploymentFixture() *appsv1.Deployment {
	replicas := int32(1)
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "demo", UID: "d-1", Generation: 1, Annotations: map[string]string{revisionAnnotation: "1"}}, Spec: appsv1.DeploymentSpec{Replicas: &replicas, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1.27"}}}}}, Status: appsv1.DeploymentStatus{ObservedGeneration: 1, Replicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1, Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"}}}}
	return d
}
func testController(t *testing.T) *controller {
	t.Helper()
	c := newController(controllerOptions{clusterUID: "cluster-1", logger: slog.New(slog.NewTextHandler(io.Discard, nil)), now: func() time.Time { return time.Unix(100, 0) }})
	t.Cleanup(c.queue.ShutDown)
	t.Cleanup(c.imageQueue.ShutDown)
	return c
}
func take(t *testing.T, c *controller, status string) (*eventBatch, rolloutEvent) {
	t.Helper()
	require.Positive(t, c.queue.Len())
	event, _ := c.queue.Get()
	c.queue.Done(event)
	record := event.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, eventPrefix+status, record.EventName())
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(record.Body().Str()), &fields))
	require.NotContains(t, fields, "change_types")
	var payload rolloutEvent
	require.NoError(t, json.Unmarshal([]byte(record.Body().Str()), &payload))
	require.Equal(t, status, payload.Status)
	require.NotEmpty(t, payload.RolloutID)
	return event, payload
}
func revision(d *appsv1.Deployment, n int64) {
	d.Generation++
	d.Status.ObservedGeneration = d.Generation
	d.Annotations[revisionAnnotation] = strconv.FormatInt(n, 10)
	d.Status.Conditions = nil
	d.Status.AvailableReplicas = 0
}
func succeed(d *appsv1.Deployment) {
	d.Status.ObservedGeneration = d.Generation
	d.Status.Replicas = *d.Spec.Replicas
	d.Status.UpdatedReplicas = *d.Spec.Replicas
	d.Status.AvailableReplicas = *d.Spec.Replicas
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"}}
}
func TestRolloutLifecycle(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, true)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	// Scaling changes generation, not revision, and cannot reopen a completed rollout.
	d.Generation++
	*d.Spec.Replicas = 2
	c.observe(d, false)
	succeed(d)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	c.observe(d, false)
	_, started := take(t, c, "started")
	require.Equal(t, int64(2), started.Revision)
	require.Len(t, started.Changes, 1)
	require.Equal(t, "image", started.Changes[0].Field)
	require.True(t, *started.ChangesKnown)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
	c.observe(d, false)
	_, stalled := take(t, c, "stalled")
	require.Equal(t, started.RolloutID, stalled.RolloutID)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	succeed(d)
	c.observe(d, false)
	_, completed := take(t, c, "succeeded")
	require.Equal(t, started.RolloutID, completed.RolloutID)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	// Later availability degradation isn't a failure of this completed rollout.
	d.Status.AvailableReplicas = 0
	d.Status.Conditions[0].Status = corev1.ConditionFalse
	d.Status.Conditions[0].Reason = "ProgressDeadlineExceeded"
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
}
func TestSupersededAndRollback(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, true)
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "broken:v2"
	c.observe(d, false)
	_, old := take(t, c, "started")
	// Rollback reuses the old template, but receives a new controller revision.
	revision(d, 3)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.27"
	c.observe(d, false)
	_, superseded := take(t, c, "superseded")
	_, next := take(t, c, "started")
	require.Equal(t, old.RolloutID, superseded.RolloutID)
	require.Equal(t, next.RolloutID, superseded.SupersededBy)
	require.Equal(t, "broken:v2", superseded.Containers[0].Image)
	require.NotEqual(t, old.RolloutID, next.RolloutID)
	require.Equal(t, "broken:v2", *next.Changes[0].Before)
	require.Equal(t, "nginx:1.27", *next.Changes[0].After)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	revision(d, 2)
	c.observe(d, false)
	require.Zero(t, c.queue.Len(), "stale revision must not supersede current rollout")
}
func TestPausedAndUnobservedChanges(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, true)
	d.Spec.Paused = true
	d.Generation++
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	d.Status.ObservedGeneration = d.Generation
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	d.Spec.Paused = false
	d.Generation++
	d.Annotations[revisionAnnotation] = "2"
	c.observe(d, false)
	require.Zero(t, c.queue.Len(), "old controller status must not complete a new revision")
	d.Status.ObservedGeneration = d.Generation
	d.Status.Conditions = nil
	c.observe(d, false)
	_, started := take(t, c, "started")
	require.Equal(t, "nginx:1.27", *started.Changes[0].Before)
	require.Zero(t, c.queue.Len())
}
func TestNewDeploymentAndBaseline(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, false)
	_, started := take(t, c, "started")
	take(t, c, "succeeded")
	require.False(t, *started.ChangesKnown)
	require.Empty(t, started.Changes)
	restarted := testController(t)
	restarted.observe(d, true)
	restarted.observe(d, false)
	require.Zero(t, restarted.queue.Len())
	// A rollout already in flight at startup can finish without replaying started.
	revision(d, 2)
	restarted.observe(d, true)
	require.Zero(t, restarted.queue.Len())
	succeed(d)
	restarted.observe(d, false)
	take(t, restarted, "succeeded")
	// Same revision number in a recreated Deployment must have a different ID.
	d.UID = "d-2"
	c.observe(d, false)
	_, recreated := take(t, c, "started")
	take(t, c, "succeeded")
	require.NotEqual(t, started.RolloutID, recreated.RolloutID)
}
func TestDeliveryRetries(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, false)
	event, _ := take(t, c, "started")
	expected := event.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str()
	c.opts.emit = func(_ context.Context, batch func() eventBatch) error {
		data := batch()
		data.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("mutated")
		return errors.New("retry")
	}
	require.False(t, c.deliver(t.Context(), event))
	c.opts.emit = func(_ context.Context, batch func() eventBatch) error {
		data := batch()
		require.Equal(t, event.id, data.id)
		require.Equal(t, expected, data.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
		return nil
	}
	require.True(t, c.deliver(t.Context(), event))
}
func TestWatcherUsesDeploymentReplicaSetAndPodWatches(t *testing.T) {
	d := deploymentFixture()
	client := fake.NewClientset(d)
	received := make(chan plog.Logs, 8)
	c := newController(controllerOptions{clusterUID: "explicit-cluster", client: client, emit: func(_ context.Context, batch func() eventBatch) error { received <- batch().logs; return nil }})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.run(ctx) }()
	require.Eventually(t, func() bool {
		for _, action := range client.Actions() {
			if action.GetVerb() == "watch" && action.GetResource().Resource == "deployments" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	select {
	case logs := <-received:
		require.Equal(t, "grafana.sdlc.k8s.cluster.heartbeat", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
	case <-time.After(3 * time.Second):
		t.Fatal("no initial heartbeat after cache sync")
	}
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	_, err := client.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	require.NoError(t, err)
	select {
	case logs := <-received:
		require.Equal(t, eventPrefix+"started", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
	case <-time.After(3 * time.Second):
		t.Fatal("no rollout event")
	}
	for _, action := range client.Actions() {
		require.Contains(t, []string{"deployments", "replicasets", "pods"}, action.GetResource().Resource)
	}
	cancel()
	require.NoError(t, <-done)
}

func TestDeploymentDeletion(t *testing.T) {
	c := testController(t)
	d := deploymentFixture()
	// Initial discovery is silent, but deletion must still be reported.
	c.observe(d, true)
	c.remove(cache.DeletedFinalStateUnknown{Obj: d})
	require.Empty(t, c.rollouts)
	require.Empty(t, c.collections)
	batch, _ := c.queue.Get()
	c.queue.Done(batch)
	record := batch.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "grafana.sdlc.k8s.deployment.deleted", record.EventName())
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(record.Body().Str()), &payload))
	require.Equal(t, map[string]any{"name": "web", "uid": "d-1", "observed_at": "1970-01-01T00:01:40Z"}, payload)
	require.Equal(t, stableID(stableID("cluster-1", "d-1"), "deleted"), batch.id)
	// Retry/tombstone delivery uses the same identity, even with no rollout state.
	c.remove(d)
	repeated, _ := c.queue.Get()
	c.queue.Done(repeated)
	require.Equal(t, batch.id, repeated.id)
	// A recreated Deployment with the same name is a different identity.
	d.UID = "d-2"
	c.remove(d)
	recreated, _ := c.queue.Get()
	c.queue.Done(recreated)
	require.NotEqual(t, batch.id, recreated.id)
	c.remove(cache.DeletedFinalStateUnknown{Obj: "unavailable"})
	require.Zero(t, c.queue.Len())
}
