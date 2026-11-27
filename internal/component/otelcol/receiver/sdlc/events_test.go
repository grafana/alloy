package sdlc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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
	context   map[string]any
	summary   string
	rolloutID string
	revision  int64
}

// meta is the metadata of a snapshot side of the diff, or nil.
func (d decoded) meta(side string) map[string]any {
	snapshot, _ := d.diff[side].(map[string]any)
	m, _ := snapshot["metadata"].(map[string]any)
	return m
}

// image is the image of the first container in a workload snapshot side of
// the diff, or nil.
func (d decoded) image(side string) any {
	snapshot, _ := d.diff[side].(map[string]any)
	spec, _ := snapshot["spec"].(map[string]any)
	template, _ := spec["template"].(map[string]any)
	podSpec, _ := template["spec"].(map[string]any)
	containers, _ := podSpec["containers"].([]any)
	if len(containers) == 0 {
		return nil
	}
	return containers[0].(map[string]any)["image"]
}

func decode(t *testing.T, logs plog.Logs) decoded {
	t.Helper()
	rl := logs.ResourceLogs().At(0)
	record := rl.ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, eventName, record.EventName())
	out := decoded{record: record, resource: rl.Resource().Attributes().AsRaw(), attrs: record.Attributes().AsRaw()}
	out.diff, _ = out.attrs["grafana.sdlc.change.diff"].(map[string]any)
	out.context, _ = out.attrs["grafana.sdlc.change.context"].(map[string]any)
	for key := range out.diff {
		require.Contains(t, []string{"before_known", "before", "after", "snapshots_omitted"}, key, "change.diff holds only snapshots")
	}
	out.summary, _ = out.attrs["grafana.sdlc.change.summary"].(string)
	out.rolloutID, _ = out.context["rollout_id"].(string)
	out.revision, _ = out.context["revision"].(int64)
	return out
}

var summaryFor = map[string]string{
	rolloutStarted:        "Rollout started",
	rolloutSucceeded:      "Rollout succeeded",
	rolloutStalled:        "Rollout stalled",
	rolloutSuperseded:     "Rollout superseded",
	rolloutImagesResolved: "Resolved runtime images",
}

// nextEvent takes the next queued event and checks the attributes every event
// has.
func nextEvent(t *testing.T, c *controller) (*eventBatch, decoded) {
	t.Helper()
	require.Positive(t, c.queue.Len())
	event, _ := c.queue.Get()
	c.queue.Done(event)
	got := decode(t, event.logs)
	require.Equal(t, "UNSET", got.attrs["grafana.sdlc.status"])
	require.Equal(t, "1", got.attrs["grafana.sdlc.schema.version"])
	require.Equal(t, event.id, got.attrs["grafana.sdlc.event.id"])
	require.NotEmpty(t, got.attrs["grafana.sdlc.provider.version"])
	require.True(t, got.record.SpanID().IsEmpty())
	return event, got
}

// take takes the next event, for the demo/web Deployment.
func take(t *testing.T, c *controller, status string) (*eventBatch, decoded) {
	t.Helper()
	event, got := nextEvent(t, c)
	requireRollout(t, got, "deployment", status)
	return event, got
}

// takeWorkload takes the next event, for a demo/web workload of resourceType.
func takeWorkload(t *testing.T, c *controller, resourceType, status string) decoded {
	t.Helper()
	_, got := nextEvent(t, c)
	requireRollout(t, got, resourceType, status)
	return got
}

func requireRollout(t *testing.T, got decoded, resourceType, status string) {
	t.Helper()
	require.Contains(t, got.summary, summaryFor[status])
	require.NotEmpty(t, got.rolloutID)
	require.False(t, got.record.TraceID().IsEmpty())
	// The resource names the workload; the summary must not.
	require.Equal(t, map[string]any{
		"grafana.sdlc.provider.type":                 "k8s",
		"grafana.sdlc.provider.id":                   "cluster-1",
		"grafana.sdlc.provider.resource.type":        resourceType,
		"grafana.sdlc.provider.resource.id":          "demo/web",
		"grafana.sdlc.provider.resource.change.type": "rollout_" + status,
	}, got.resource)
	require.NotContains(t, got.summary, "web")
}

// runWatcher runs a controller with opts until the test ends, and returns the
// events it delivers.
func runWatcher(t *testing.T, opts controllerOptions) <-chan plog.Logs {
	t.Helper()
	received := make(chan plog.Logs, 8)
	opts.emit = func(_ context.Context, batch func() eventBatch) error { received <- batch().logs; return nil }
	c := newController(opts)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- c.run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	return received
}

// requireWatching waits until actions include a watch of resource.
func requireWatching(t *testing.T, actions func() []k8stesting.Action, resource string) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, action := range actions() {
			if action.GetVerb() == "watch" && action.GetResource().Resource == resource {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

// nextReceived waits for the next delivered event.
func nextReceived(t *testing.T, received <-chan plog.Logs) decoded {
	t.Helper()
	select {
	case logs := <-received:
		return decode(t, logs)
	case <-time.After(3 * time.Second):
		t.Fatal("no event")
		return decoded{}
	}
}

// nextAdded waits for the next received event about the demo/new resource,
// skipping events of other resources.
func nextAdded(t *testing.T, received <-chan plog.Logs) decoded {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case logs := <-received:
			if got := decode(t, logs); got.resource["grafana.sdlc.provider.resource.id"] == "demo/new" {
				return got
			}
		case <-timeout:
			t.Fatal("no event for demo/new")
		}
	}
}

// requireCreated asserts got reports the creation of a workload of resourceType.
func requireCreated(t *testing.T, got decoded, resourceType string) {
	t.Helper()
	require.Equal(t, resourceType, got.resource["grafana.sdlc.provider.resource.type"])
	require.Equal(t, "created", got.resource["grafana.sdlc.provider.resource.change.type"])
	require.Equal(t, "Created", got.summary)
	require.Equal(t, "effect", got.attrs["grafana.sdlc.type"])
	require.Equal(t, "1", got.attrs["grafana.sdlc.change.version.after"])
	require.NotContains(t, got.attrs, "grafana.sdlc.change.version.before")
	// A creation has only the template it was created with.
	require.Equal(t, []string{"after"}, slices.Collect(maps.Keys(got.diff)))
	after, _ := got.diff["after"].(map[string]any)
	require.Contains(t, resourceKinds, resourceKind{resourceType: resourceType, apiVersion: "apps/v1", kind: after["kind"].(string)})
	require.Equal(t, "new", after["metadata"].(map[string]any)["name"])
	require.Contains(t, after["spec"], "template")
	require.True(t, got.record.TraceID().IsEmpty(), "creation isn't part of a rollout trace")
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
	c.observe(d, observedInitial)
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	// Scaling changes generation, not revision, and cannot reopen a completed rollout.
	d.Generation++
	*d.Spec.Replicas = 2
	c.observe(d, observedUpdated)
	succeed(d)
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	c.observe(d, observedUpdated)
	_, started := take(t, c, "started")
	require.Equal(t, int64(2), started.revision)
	require.Equal(t, "intent", started.attrs["grafana.sdlc.type"])
	require.Equal(t, "1", started.attrs["grafana.sdlc.change.version.before"])
	require.Equal(t, "2", started.attrs["grafana.sdlc.change.version.after"])
	require.Equal(t, true, started.diff["before_known"])
	require.Equal(t, "nginx:1.27", started.image("before"))
	require.Equal(t, "nginx:1.28", started.image("after"))
	// Snapshots have the shape of the workload, without bookkeeping metadata.
	after := started.diff["after"].(map[string]any)
	require.Equal(t, "apps/v1", after["apiVersion"])
	require.Equal(t, "Deployment", after["kind"])
	require.Equal(t, map[string]any{"name": "web", "namespace": "demo", "uid": "d-1"}, after["metadata"],
		"the revision annotation repeats the event version")
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
	c.observe(d, observedUpdated)
	_, stalled := take(t, c, "stalled")
	require.Equal(t, started.rolloutID, stalled.rolloutID)
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	succeed(d)
	c.observe(d, observedUpdated)
	_, completed := take(t, c, "succeeded")
	require.Equal(t, started.rolloutID, completed.rolloutID)
	require.Equal(t, "effect", completed.attrs["grafana.sdlc.type"])
	// Every event of a rollout shares one trace.
	require.Equal(t, started.record.TraceID(), completed.record.TraceID())
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	// Later availability degradation isn't a failure of this completed rollout.
	d.Status.AvailableReplicas = 0
	d.Status.Conditions[0].Status = corev1.ConditionFalse
	d.Status.Conditions[0].Reason = "ProgressDeadlineExceeded"
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
}
func TestSupersededAndRollback(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, observedInitial)
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "broken:v2"
	c.observe(d, observedUpdated)
	_, old := take(t, c, "started")
	// Rollback reuses the old template, but receives a new controller revision.
	revision(d, 3)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.27"
	c.observe(d, observedUpdated)
	_, superseded := take(t, c, "superseded")
	_, next := take(t, c, "started")
	require.Equal(t, old.rolloutID, superseded.rolloutID)
	require.Contains(t, superseded.summary, "revision 3")
	require.Equal(t, next.rolloutID, superseded.context["superseded_by"])
	require.Equal(t, "broken:v2", superseded.context["containers"].([]any)[0].(map[string]any)["image"])
	require.NotEqual(t, old.rolloutID, next.rolloutID)
	require.Equal(t, "broken:v2", next.image("before"))
	require.Equal(t, "nginx:1.27", next.image("after"))
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	revision(d, 2)
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len(), "stale revision must not supersede current rollout")
}
func TestPausedAndUnobservedChanges(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, observedInitial)
	d.Spec.Paused = true
	d.Generation++
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	d.Status.ObservedGeneration = d.Generation
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	d.Spec.Paused = false
	d.Generation++
	d.Annotations[revisionAnnotation] = "2"
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len(), "old controller status must not complete a new revision")
	d.Status.ObservedGeneration = d.Generation
	d.Status.Conditions = nil
	c.observe(d, observedUpdated)
	_, started := take(t, c, "started")
	require.Equal(t, "nginx:1.27", started.image("before"))
	require.Zero(t, c.queue.Len())
}
func TestNewDeploymentAndBaseline(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, observedUpdated)
	_, started := take(t, c, "started")
	take(t, c, "succeeded")
	require.Equal(t, false, started.diff["before_known"])
	require.NotContains(t, started.diff, "before")
	require.Contains(t, started.diff, "after")
	restarted := testController(t)
	restarted.observe(d, observedInitial)
	restarted.observe(d, observedUpdated)
	require.Zero(t, restarted.queue.Len())
	// A rollout already in flight at startup can finish without replaying started.
	revision(d, 2)
	restarted.observe(d, observedInitial)
	require.Zero(t, restarted.queue.Len())
	succeed(d)
	restarted.observe(d, observedUpdated)
	take(t, restarted, "succeeded")
	// Same revision number in a recreated Deployment must have a different ID.
	d.UID = "d-2"
	c.observe(d, observedUpdated)
	_, recreated := take(t, c, "started")
	take(t, c, "succeeded")
	require.NotEqual(t, started.rolloutID, recreated.rolloutID)
}
func TestOversizedRolloutOmitsSnapshots(t *testing.T) {
	d := deploymentFixture()
	// Each snapshot holds the unchanged selector, so together they exceed the limit.
	d.Spec.Template.Spec.NodeSelector = map[string]string{"pool": strings.Repeat("x", maxEventBytes/2)}
	c := testController(t)
	c.observe(d, observedInitial)
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	c.observe(d, observedUpdated)
	event, started := take(t, c, "started")
	require.Equal(t, "size", started.diff["snapshots_omitted"])
	require.NotContains(t, started.context, "snapshots_omitted")
	require.NotContains(t, started.diff, "before")
	require.NotContains(t, started.diff, "after")
	require.Equal(t, true, started.diff["before_known"])
	size, err := wireSize(event.logs)
	require.NoError(t, err)
	require.LessOrEqual(t, size, maxEventBytes)
}
func TestDeliveryRetries(t *testing.T) {
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, observedUpdated)
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
	received := runWatcher(t, controllerOptions{clusterUID: "explicit-cluster", client: client})
	requireWatching(t, client.Actions, "deployments")
	require.Empty(t, received, "initial observation is a silent baseline")
	revision(d, 2)
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	_, err := client.AppsV1().Deployments(d.Namespace).Update(t.Context(), d, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Contains(t, nextReceived(t, received).summary, "Rollout started")
	// A Deployment added after the initial listing is reported as created, then
	// as its first rollout.
	added := deploymentFixture()
	added.Name, added.UID = "new", "d-2"
	_, err = client.AppsV1().Deployments(added.Namespace).Create(t.Context(), added, metav1.CreateOptions{})
	require.NoError(t, err)
	requireCreated(t, nextAdded(t, received), "deployment")
	started := nextAdded(t, received)
	require.Equal(t, "rollout_started", started.resource["grafana.sdlc.provider.resource.change.type"])
	require.Equal(t, false, started.diff["before_known"])
	for _, action := range client.Actions() {
		require.Contains(t, []string{"deployments", "replicasets", "pods"}, action.GetResource().Resource)
	}
}

func TestChangeContextHasNoPerFieldChanges(t *testing.T) {
	// Consumers compare the snapshots; earlier versions listed changed fields.
	d := deploymentFixture()
	c := testController(t)
	c.observe(d, observedUpdated)
	_, started := take(t, c, "started")
	require.NotContains(t, started.context, "changes")
	require.NotContains(t, started.context, "changes_known")
}

func TestDeploymentDeletionClearsTrackingWithoutEvent(t *testing.T) {
	c := testController(t)
	d := deploymentFixture()
	c.observe(d, observedInitial)
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
	c.observe(d, observedInitial)
	revision(d, 2)
	c.observe(d, observedUpdated)
	_, started := take(t, c, "started")
	clock = clock.Add(30 * time.Second)
	succeed(d)
	c.observe(d, observedUpdated)
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
		c.observe(d, observedUpdated)
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
	c.observe(d, observedUpdated)
	event, _ := c.queue.Get()
	got := decode(t, event.logs)
	require.Equal(t, "prod-eu", got.resource["grafana.sdlc.provider.id"])
	require.Equal(t, "prod-eu", got.context["cluster_name"])
}
