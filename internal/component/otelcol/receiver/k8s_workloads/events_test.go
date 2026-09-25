package k8s_workloads

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

type capture struct {
	mu   sync.Mutex
	logs []plog.Logs
	fail bool
}

func (s *capture) emit(_ context.Context, batch func() eventBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs := batch().logs
	if s.fail {
		return errors.New("unavailable")
	}
	s.logs = append(s.logs, logs)
	return nil
}
func (s *capture) records() []plog.LogRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []plog.LogRecord{}
	for _, logs := range s.logs {
		out = append(out, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0))
	}
	return out
}
func body(t *testing.T, r plog.LogRecord) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.Body().Str()), &out))
	return out
}
func fixture() (*corev1.Namespace, *appsv1.Deployment, *appsv1.ReplicaSet, *corev1.Pod) {
	yes := true
	n := int32(1)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo", UID: "ns-1"}}
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: ns.Name, UID: "d-1", Generation: 1}, Spec: appsv1.DeploymentSpec{Replicas: &n, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1.27"}}}}}, Status: appsv1.DeploymentStatus{ObservedGeneration: 1, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}}
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "web-rs", Namespace: ns.Name, UID: "rs-1", OwnerReferences: []metav1.OwnerReference{{UID: d.UID, Controller: &yes}}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-pod", Namespace: ns.Name, UID: "p-1", OwnerReferences: []metav1.OwnerReference{{UID: rs.UID, Controller: &yes}}}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "app", ImageID: "containerd://sha256:abcd"}}}}
	return ns, d, rs, pod
}
func testController(t *testing.T, objects ...runtime.Object) (*controller, *capture, *fake.Clientset) {
	t.Helper()
	sink := &capture{}
	client := fake.NewClientset(objects...)
	c := newController(controllerOptions{client: client, clusterUID: "cluster", emit: sink.emit, now: func() time.Time { return time.Unix(100, 0) }})
	t.Cleanup(c.queue.ShutDown)
	return c, sink, client
}
func TestCompleteSnapshotsAndImages(t *testing.T) {
	ns, d, rs, pod := fixture()
	empty := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "empty", UID: "ns-2"}}
	c, sink, _ := testController(t, ns, d, rs, pod, empty)
	c.collect(t.Context())
	records := sink.records()
	require.Len(t, records, 3)
	require.Equal(t, eventPrefix+"namespace.snapshot", records[0].EventName())
	require.Len(t, body(t, records[0])["namespaces"], 2)
	entries := body(t, records[1])["deployments"].([]any)
	require.Len(t, entries, 1)
	entry := entries[0].(map[string]any)
	require.Equal(t, "healthy", entry["status"])
	require.Equal(t, "d-1", entry["uid"])
	image := entry["containers"].([]any)[0].(map[string]any)
	require.Equal(t, "nginx:1.27", image["image"])
	require.Equal(t, "sha256:abcd", image["resolved"].([]any)[0].(map[string]any)["digest"])
	require.Equal(t, []any{}, body(t, records[2])["deployments"])
	require.Equal(t, true, body(t, records[1])["complete"])
	// A second scan emits even unchanged state, with a new collection timestamp.
	c.now = func() time.Time { return time.Unix(160, 0) }
	c.collect(t.Context())
	records = sink.records()
	require.Len(t, records, 6)
	first, _ := records[1].Attributes().Get("grafana.sdlc.event.id")
	second, _ := records[4].Attributes().Get("grafana.sdlc.event.id")
	require.NotEqual(t, first.Str(), second.Str())
}
func TestEmptyClusterAndCollectionFailures(t *testing.T) {
	c, sink, client := testController(t)
	c.collect(t.Context())
	require.Equal(t, []any{}, body(t, sink.records()[0])["namespaces"])
	client.PrependReactor("list", "namespaces", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("forbidden") })
	c.collect(t.Context())
	records := sink.records()
	require.Len(t, records, 2)
	require.Equal(t, errorEventName, records[1].EventName())
	require.Equal(t, "collection_failed", body(t, records[1])["reason"])
}
func TestEnrichmentFailureNeverPublishesPartialInventory(t *testing.T) {
	ns, d, _, _ := fixture()
	c, sink, client := testController(t, ns, d)
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("forbidden") })
	c.collect(t.Context())
	records := sink.records()
	require.Len(t, records, 2)
	require.Equal(t, eventPrefix+"namespace.snapshot", records[0].EventName())
	require.Equal(t, errorEventName, records[1].EventName())
}
func TestPayloadLimitAtBoundary(t *testing.T) {
	c, sink, _ := testController(t)
	event := makeEvent("cluster", "", "demo", "ns", "", "snapshot", "deployment", c.now(), map[string]any{"deployments": []any{}, "padding": strings.Repeat("a", 9000)})
	raw, err := (&plog.JSONMarshaler{}).MarshalLogs(event.logs)
	require.NoError(t, err)
	protobuf, err := (&plog.ProtoMarshaler{}).MarshalLogs(event.logs)
	require.NoError(t, err)
	size := max(len(raw), len(protobuf))
	c.opts.snapshots.MaxSizeBytes = size
	require.NoError(t, c.deliver(t.Context(), event))
	require.Len(t, sink.records(), 1)
	errorRecord := sink.records()[0]
	require.Equal(t, errorEventName, errorRecord.EventName())
	require.Equal(t, float64(size), body(t, errorRecord)["measured_bytes"])
	require.NotContains(t, errorRecord.Body().Str(), "padding")
	c.opts.snapshots.MaxSizeBytes = size + 1
	require.NoError(t, c.deliver(t.Context(), event))
	require.Len(t, sink.records(), 2)
	require.Equal(t, eventPrefix+"deployment.snapshot", sink.records()[1].EventName())
}
func TestFailedDeliveryRetainsOriginalPayload(t *testing.T) {
	c, sink, _ := testController(t)
	sink.fail = true
	event := makeEvent("cluster", "", "demo", "ns", "d", "created", "deployment", c.now(), map[string]any{"uid": "d"})
	before, err := (&plog.JSONMarshaler{}).MarshalLogs(event.logs)
	require.NoError(t, err)
	require.Error(t, c.deliver(t.Context(), event))
	// Error reporting failure must not recurse or enqueue more error events.
	c.report(t.Context(), event, "delivery_failed", "Delivery failed", 0, 0)
	require.Empty(t, sink.records())
	sink.fail = false
	c.now = func() time.Time { return time.Unix(999, 0) }
	require.NoError(t, c.deliver(t.Context(), event))
	after, err := (&plog.JSONMarshaler{}).MarshalLogs(sink.logs[0])
	require.NoError(t, err)
	require.Equal(t, before, after)
}
func TestNamespaceRecreationDoesNotProduceWrongScopeSnapshot(t *testing.T) {
	ns, _, _, _ := fixture()
	c, sink, client := testController(t, ns)
	client.PrependReactor("get", "namespaces", func(ktesting.Action) (bool, runtime.Object, error) {
		copy := ns.DeepCopy()
		copy.UID = "new-uid"
		return true, copy, nil
	})
	c.collect(t.Context())
	require.Equal(t, errorEventName, sink.records()[1].EventName())
}
func TestNotificationsAndTombstones(t *testing.T) {
	ns, d, _, _ := fixture()
	c, _, _ := testController(t)
	for _, tc := range []struct {
		kind string
		obj  any
	}{{"namespace", ns}, {"deployment", d}} {
		for _, operation := range []string{"created", "deleted"} {
			obj := tc.obj
			if operation == "deleted" {
				obj = cache.DeletedFinalStateUnknown{Obj: obj}
			}
			c.notify(tc.kind, operation, obj)
			item, shutdown := c.queue.Get()
			require.False(t, shutdown)
			require.Equal(t, eventPrefix+tc.kind+"."+operation, item.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
			c.queue.Done(item)
		}
	}
}
func TestStartupTickAndWatchEvents(t *testing.T) {
	ns, d, _, _ := fixture()
	c, sink, client := testController(t, ns, d)
	c.opts.snapshots.Interval = 50 * time.Millisecond
	c.now = time.Now
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.run(ctx) }()
	require.Eventually(t, func() bool { return len(sink.records()) >= 4 }, 3*time.Second, 10*time.Millisecond)
	for _, r := range sink.records() {
		require.True(t, strings.HasSuffix(r.EventName(), ".snapshot"), "initial list must not invent created notifications")
	}
	created := ns.DeepCopy()
	created.Name = "new"
	created.UID = types.UID("new-ns")
	_, err := client.CoreV1().Namespaces().Create(ctx, created, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		for _, r := range sink.records() {
			if r.EventName() == eventPrefix+"namespace.created" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, client.AppsV1().Deployments(ns.Name).Delete(ctx, d.Name, metav1.DeleteOptions{}))
	require.Eventually(t, func() bool {
		for _, r := range sink.records() {
			if r.EventName() == eventPrefix+"deployment.deleted" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	cancel()
	require.NoError(t, <-done)
}
func TestDeploymentStatus(t *testing.T) {
	_, d, _, _ := fixture()
	require.Equal(t, "healthy", deploymentStatus(d))
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
	require.Equal(t, "stalled", deploymentStatus(d))
	d.Generation++
	require.Equal(t, "progressing", deploymentStatus(d))
	d.Spec.Paused = true
	require.Equal(t, "paused", deploymentStatus(d))
	now := metav1.Now()
	d.DeletionTimestamp = &now
	require.Equal(t, "terminating", deploymentStatus(d))
}

func TestRestartRecoversMissedDeploymentAndNamespaceDeletions(t *testing.T) {
	ns, d, _, _ := fixture()
	other := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other", UID: "other-uid"}}
	c, sink, client := testController(t, ns, d, other)
	c.collect(t.Context())
	require.NoError(t, client.AppsV1().Deployments(ns.Name).Delete(t.Context(), d.Name, metav1.DeleteOptions{}))
	require.NoError(t, client.CoreV1().Namespaces().Delete(t.Context(), other.Name, metav1.DeleteOptions{}))
	restarted := newController(c.opts)
	t.Cleanup(restarted.queue.ShutDown)
	restarted.now = func() time.Time { return time.Unix(200, 0) }
	before := len(sink.records())
	restarted.collect(t.Context())
	records := sink.records()[before:]
	require.Len(t, records, 2)
	require.Len(t, body(t, records[0])["namespaces"], 1)
	require.Equal(t, []any{}, body(t, records[1])["deployments"])
}

func TestOversizedNamespaceDoesNotBlockOtherSnapshots(t *testing.T) {
	ns, d, _, _ := fixture()
	d.Labels = map[string]string{"large": strings.Repeat("x", 9000)}
	empty := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other", UID: "other-uid"}}
	c, sink, _ := testController(t, ns, d, empty)
	c.opts.snapshots.MaxSizeBytes = 8192
	c.collect(t.Context())
	records := sink.records()
	require.Len(t, records, 3)
	require.Equal(t, errorEventName, records[1].EventName())
	require.Equal(t, []any{}, body(t, records[2])["deployments"])
}

func TestImagesAcrossReplicaSetsAndInitContainers(t *testing.T) {
	_, d, rs, pod := fixture()
	d.Spec.Template.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "example/init@sha256:pinned"}}
	secondRS := rs.DeepCopy()
	secondRS.UID = "rs-2"
	secondPod := pod.DeepCopy()
	secondPod.Name = "second"
	secondPod.OwnerReferences[0].UID = secondRS.UID
	secondPod.Status.ContainerStatuses[0].ImageID = "containerd://sha256:other"
	secondPod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init", ImageID: "containerd://sha256:init"}}
	entry := describeDeployment(d, []appsv1.ReplicaSet{*rs, *secondRS}, []corev1.Pod{*pod, *secondPod})
	require.Len(t, entry.Containers, 2)
	require.Len(t, entry.Containers[0].Resolved, 2)
	require.True(t, entry.Containers[1].Init)
	require.Equal(t, "sha256:init", entry.Containers[1].Resolved[0].Digest)
}

func TestTerminalNotifications(t *testing.T) {
	_, d, _, _ := fixture()
	c, _, _ := testController(t)
	take := func(operation string, generation int64) {
		t.Helper()
		require.Equal(t, 1, c.queue.Len())
		event, _ := c.queue.Get()
		defer c.queue.Done(event)
		record := event.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		require.Equal(t, eventPrefix+"deployment."+operation, record.EventName())
		payload := body(t, record)
		require.Equal(t, string(d.UID), payload["uid"])
		require.Equal(t, float64(generation), payload["generation"])
		require.NotContains(t, payload, "containers")
	}
	// Initial terminal state is represented by the startup snapshot.
	c.observeTerminal(d, true)
	c.observeTerminal(d, false)
	require.Zero(t, c.queue.Len())
	// HPA updates generation, but not the Pod template.
	d.Generation++
	*d.Spec.Replicas = 2
	c.observeTerminal(d, false)
	d.Status.ObservedGeneration = d.Generation
	d.Status.Replicas, d.Status.UpdatedReplicas, d.Status.AvailableReplicas = 2, 2, 2
	c.observeTerminal(d, false)
	require.Zero(t, c.queue.Len())
	// A new template must not inherit success from stale controller status.
	d.Generation++
	d.Spec.Template.Spec.Containers[0].Image = "nginx:1.28"
	c.observeTerminal(d, false)
	require.Zero(t, c.queue.Len())
	d.Status.ObservedGeneration = d.Generation
	c.observeTerminal(d, false)
	take("succeeded", 3)
	c.observeTerminal(d, false)
	require.Zero(t, c.queue.Len())
	// A stalled rollout and its recovery are distinct terminal transitions.
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
	c.observeTerminal(d, false)
	take("stalled", 3)
	c.observeTerminal(d, false)
	require.Zero(t, c.queue.Len())
	d.Status.Conditions = nil
	c.observeTerminal(d, false)
	take("succeeded", 3)
	c.notify("deployment", "deleted", d)
	require.NotContains(t, c.terminals, string(d.UID))
}

func TestProgressingStartupEmitsTerminalNotification(t *testing.T) {
	_, d, _, _ := fixture()
	c, _, _ := testController(t)
	d.Status.AvailableReplicas = 0
	c.observeTerminal(d, true)
	require.Zero(t, c.queue.Len())
	d.Status.AvailableReplicas = 1
	c.observeTerminal(d, false)
	require.Equal(t, 1, c.queue.Len())
}
