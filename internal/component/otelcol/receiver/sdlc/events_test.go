package sdlc

import (
	"context"
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

// decoded is a grafana.sdlc.change record flattened for assertions.
type decoded struct {
	record    plog.LogRecord
	resource  map[string]any
	attrs     map[string]any
	diff      map[string]any
	summary   string
	rolloutID string
	revision  int64
}

func (d decoded) changes() map[string]any {
	changes, _ := d.diff["changes"].(map[string]any)
	return changes
}

func decode(t *testing.T, logs plog.Logs) decoded {
	t.Helper()
	rl := logs.ResourceLogs().At(0)
	record := rl.ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, eventName, record.EventName())
	out := decoded{record: record, resource: rl.Resource().Attributes().AsRaw(), attrs: record.Attributes().AsRaw()}
	out.diff, _ = out.attrs["grafana.sdlc.change.diff"].(map[string]any)
	out.summary, _ = out.attrs["grafana.sdlc.change.summary"].(string)
	out.rolloutID, _ = out.diff["rollout_id"].(string)
	out.revision, _ = out.diff["revision"].(int64)
	return out
}

var summaryFor = map[string]string{
	rolloutStarted:        "Rollout started",
	rolloutSucceeded:      "Rollout succeeded",
	rolloutStalled:        "Rollout stalled",
	rolloutSuperseded:     "Rollout superseded",
	rolloutImagesResolved: "Resolved runtime images",
}

func take(t *testing.T, c *controller, status string) (*eventBatch, decoded) {
	t.Helper()
	require.Positive(t, c.queue.Len())
	event, _ := c.queue.Get()
	c.queue.Done(event)
	got := decode(t, event.logs)
	require.Contains(t, got.summary, summaryFor[status])
	require.Equal(t, "UNSET", got.attrs["grafana.sdlc.status"])
	require.Equal(t, "1", got.attrs["grafana.sdlc.schema.version"])
	require.Equal(t, event.id, got.attrs["grafana.sdlc.event.id"])
	require.NotEmpty(t, got.attrs["grafana.sdlc.provider.version"])
	require.NotEmpty(t, got.rolloutID)
	require.False(t, got.record.TraceID().IsEmpty())
	require.True(t, got.record.SpanID().IsEmpty())
	// The resource names the Deployment; the summary must not.
	require.Equal(t, map[string]any{
		"grafana.sdlc.provider.type": "k8s",
		"grafana.sdlc.provider.id":   "cluster-1",
		"grafana.sdlc.resource.type": "deployment",
		"grafana.sdlc.resource.id":   "demo/web",
	}, got.resource)
	require.NotContains(t, got.summary, "web")
	return event, got
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
	require.Equal(t, int64(2), started.revision)
	require.Equal(t, "intent", started.attrs["grafana.sdlc.type"])
	require.Equal(t, "1", started.attrs["grafana.sdlc.change.version.before"])
	require.Equal(t, "2", started.attrs["grafana.sdlc.change.version.after"])
	require.Len(t, started.changes(), 1)
	require.Equal(t, map[string]any{"operation": "modified", "before": "nginx:1.27", "after": "nginx:1.28"}, started.changes()["containers.app.image"])
	require.Equal(t, true, started.diff["changes_known"])
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
	c.observe(d, false)
	_, stalled := take(t, c, "stalled")
	require.Equal(t, started.rolloutID, stalled.rolloutID)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	succeed(d)
	c.observe(d, false)
	_, completed := take(t, c, "succeeded")
	require.Equal(t, started.rolloutID, completed.rolloutID)
	require.Equal(t, "effect", completed.attrs["grafana.sdlc.type"])
	// Every event of a rollout shares one trace.
	require.Equal(t, started.record.TraceID(), completed.record.TraceID())
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
	require.Equal(t, old.rolloutID, superseded.rolloutID)
	require.Contains(t, superseded.summary, "revision 3")
	require.Equal(t, next.rolloutID, superseded.diff["superseded_by"])
	require.Equal(t, "broken:v2", superseded.diff["containers"].([]any)[0].(map[string]any)["image"])
	require.NotEqual(t, old.rolloutID, next.rolloutID)
	require.Equal(t, "broken:v2", next.changes()["containers.app.image"].(map[string]any)["before"])
	require.Equal(t, "nginx:1.27", next.changes()["containers.app.image"].(map[string]any)["after"])
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
	require.Equal(t, "nginx:1.27", started.changes()["containers.app.image"].(map[string]any)["before"])
	require.Zero(t, c.queue.Len())
}
func TestNewDeploymentAndBaseline(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, false)
	_, started := take(t, c, "started")
	take(t, c, "succeeded")
	require.Equal(t, false, started.diff["changes_known"])
	require.Empty(t, started.changes())
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
	require.NotEqual(t, started.rolloutID, recreated.rolloutID)
}
func TestDeliveryRetries(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, false)
	event, _ := take(t, c, "started")
	const summaryKey = "grafana.sdlc.change.summary"
	record := func(logs plog.Logs) plog.LogRecord {
		return logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	}
	expected, _ := record(event.logs).Attributes().Get(summaryKey)
	expectedSummary := expected.Str()
	c.opts.emit = func(_ context.Context, batch func() eventBatch) error {
		data := batch()
		record(data.logs).Attributes().PutStr(summaryKey, "mutated")
		return errors.New("retry")
	}
	require.False(t, c.deliver(t.Context(), event))
	c.opts.emit = func(_ context.Context, batch func() eventBatch) error {
		data := batch()
		require.Equal(t, event.id, data.id)
		got, _ := record(data.logs).Attributes().Get(summaryKey)
		require.Equal(t, expectedSummary, got.Str())
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
	require.Empty(t, received, "initial observation is a silent baseline")
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	_, err := client.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	require.NoError(t, err)
	select {
	case logs := <-received:
		require.Equal(t, eventName, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
		require.Contains(t, decode(t, logs).summary, "Rollout started")
	case <-time.After(3 * time.Second):
		t.Fatal("no rollout event")
	}
	for _, action := range client.Actions() {
		require.Contains(t, []string{"deployments", "replicasets", "pods"}, action.GetResource().Resource)
	}
	cancel()
	require.NoError(t, <-done)
}

func TestDeploymentDeletionClearsTrackingWithoutEvent(t *testing.T) {
	c := testController(t)
	d := deploymentFixture()
	c.observe(d, true)
	c.remove(cache.DeletedFinalStateUnknown{Obj: d})
	require.Empty(t, c.rollouts)
	require.Empty(t, c.collections)
	require.Zero(t, c.queue.Len())
	c.remove(d)
	c.remove(cache.DeletedFinalStateUnknown{Obj: "unavailable"})
	require.Zero(t, c.queue.Len())
}

func TestTerminalEventSpansRollout(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	clock := time.Unix(100, 0)
	c.now = func() time.Time { return clock }
	c.observe(d, true)
	revision(d, 2)
	c.observe(d, false)
	_, started := take(t, c, "started")
	clock = clock.Add(30 * time.Second)
	succeed(d)
	c.observe(d, false)
	_, completed := take(t, c, "succeeded")
	require.EqualValues(t, time.Unix(100, 0).UnixNano(), started.attrs["grafana.sdlc.start_time_unix_nano"])
	require.EqualValues(t, started.record.Timestamp(), time.Unix(100, 0).UnixNano())
	require.EqualValues(t, time.Unix(100, 0).UnixNano(), completed.attrs["grafana.sdlc.start_time_unix_nano"])
	require.EqualValues(t, time.Unix(130, 0).UnixNano(), completed.record.Timestamp())
}

func TestEventIsStableAcrossRebuilds(t *testing.T) {
	// Retries rebuild nothing, but IDs and trace must still be deterministic so
	// a restarted producer re-derives the same identity.
	build := func() decoded {
		d := deploymentFixture()
		c := testController(t)
		c.observe(d, false)
		event, _ := c.queue.Get()
		return decode(t, event.logs)
	}
	a, b := build(), build()
	require.Equal(t, a.attrs["grafana.sdlc.event.id"], b.attrs["grafana.sdlc.event.id"])
	require.Equal(t, a.record.TraceID(), b.record.TraceID())
}

func TestProviderIDFallsBackToClusterUID(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.opts.clusterName = "prod-eu"
	c.observe(d, false)
	event, _ := c.queue.Get()
	got := decode(t, event.logs)
	require.Equal(t, "prod-eu", got.resource["grafana.sdlc.provider.id"])
	require.Equal(t, "prod-eu", got.diff["cluster_name"])
}
