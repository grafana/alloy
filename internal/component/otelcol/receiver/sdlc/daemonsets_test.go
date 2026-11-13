package sdlc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

func daemonSetFixture() (*appsv1.DaemonSet, *appsv1.ControllerRevision) {
	d := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "demo", UID: "ds-1", Generation: 1}, Spec: appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "agent:1"}}}}}, Status: appsv1.DaemonSetStatus{ObservedGeneration: 1, DesiredNumberScheduled: 2, CurrentNumberScheduled: 2, UpdatedNumberScheduled: 2, NumberAvailable: 2}}
	return d, ownedRevision(d.UID, "DaemonSet", "web-a", 1)
}

// updateDaemonSet starts rolling out a new template.
func updateDaemonSet(d *appsv1.DaemonSet) {
	d.Generation++
	d.Status.ObservedGeneration = d.Generation
	d.Status.UpdatedNumberScheduled = 0
	d.Status.NumberAvailable = d.Status.DesiredNumberScheduled - 1
	d.Status.NumberUnavailable = 1
}

func completeDaemonSet(d *appsv1.DaemonSet) {
	d.Status.UpdatedNumberScheduled = d.Status.DesiredNumberScheduled
	d.Status.NumberAvailable = d.Status.DesiredNumberScheduled
	d.Status.NumberUnavailable = 0
}

func takeDaemonSet(t *testing.T, c *controller, status string) decoded {
	t.Helper()
	_, got := takeWorkload(t, c, status, "daemonset")
	return got
}

func TestDaemonSetRolloutLifecycle(t *testing.T) {
	d, cr := daemonSetFixture()
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observeRevisioned(d, false, true)
	c.observeRevisioned(d, false, false)
	// A new node schedules another Pod of the same revision.
	d.Status.DesiredNumberScheduled, d.Status.UpdatedNumberScheduled, d.Status.NumberAvailable = 3, 3, 3
	c.observeRevisioned(d, false, false)
	require.Zero(t, c.queue.Len())

	// The workload update arrives before its ControllerRevision.
	updateDaemonSet(d)
	d.Spec.Template.Spec.Containers[0].Image = "agent:2"
	c.observeRevisioned(d, false, false)
	require.Zero(t, c.queue.Len())
	c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
	started := takeDaemonSet(t, c, rolloutStarted)
	require.Equal(t, "1", started.attrs["grafana.sdlc.change.version.before"])
	require.Equal(t, "2", started.attrs["grafana.sdlc.change.version.after"])
	require.Equal(t, map[string]any{"operation": "modified", "before": "agent:1", "after": "agent:2"}, started.changes()["containers.app.image"])
	require.Equal(t, "RollingUpdate", started.diff["update_strategy"])
	require.NotContains(t, started.diff, "partition")

	completeDaemonSet(d)
	c.observeRevisioned(d, false, false)
	succeeded := takeDaemonSet(t, c, rolloutSucceeded)
	require.Equal(t, started.rolloutID, succeeded.rolloutID)
	c.observeRevisioned(d, false, false)
	require.Zero(t, c.queue.Len())
}

func TestDaemonSetSupersededStaleAndOnDelete(t *testing.T) {
	d, cr := daemonSetFixture()
	d.Spec.UpdateStrategy = appsv1.DaemonSetUpdateStrategy{Type: appsv1.OnDeleteDaemonSetStrategyType}
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observeRevisioned(d, false, true)
	// The API server raises the generation before the controller writes the
	// new revision and observes the generation.
	updateDaemonSet(d)
	d.Status.ObservedGeneration--
	c.observeRevisioned(d, false, false)
	c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
	require.Zero(t, c.queue.Len())
	d.Status.ObservedGeneration = d.Generation
	c.observeRevisioned(d, false, false)
	require.Equal(t, "OnDelete", takeDaemonSet(t, c, rolloutStarted).diff["update_strategy"])
	// The controller hasn't observed a newer generation yet.
	d.Generation++
	c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-c", 3))
	require.Zero(t, c.queue.Len())
	d.Status.ObservedGeneration = d.Generation
	c.observeRevisioned(d, false, false)
	require.EqualValues(t, 2, takeDaemonSet(t, c, rolloutSuperseded).revision)
	takeDaemonSet(t, c, rolloutStarted)
	// Pruning old revisions doesn't change the newest one.
	delete(c.revisions[string(d.UID)], "web-a")
	c.observeRevisioned(d, false, false)
	require.Zero(t, c.queue.Len())
	completeDaemonSet(d)
	c.observeRevisioned(d, false, false)
	takeDaemonSet(t, c, rolloutSucceeded)
}

func TestDaemonSetImages(t *testing.T) {
	pod := func(d *appsv1.DaemonSet, name, hash, image string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: d.Namespace, UID: types.UID(name), Labels: map[string]string{"app": "web", controllerRevisionHashLabel: hash}, OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", UID: d.UID, Controller: ptr.To(true)}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "app", ImageID: image}}}}
	}
	t.Run("matches revision hash", func(t *testing.T) {
		d, cr := daemonSetFixture()
		node0, node1, old := pod(d, "node-0", "b", "agent@sha256:new"), pod(d, "node-1", "b", "agent@sha256:new"), pod(d, "old", "a", "agent@sha256:old")
		c := testController(t)
		c.opts.client = fake.NewClientset(node0, node1, old)
		c.observeControllerRevision(cr)
		c.observeRevisioned(d, false, true)
		c.observePod(old)
		updateDaemonSet(d)
		c.observeRevisioned(d, false, false)
		c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
		takeDaemonSet(t, c, rolloutStarted)
		c.observePod(node0)
		completeDaemonSet(d)
		c.observeRevisioned(d, false, false)
		takeDaemonSet(t, c, rolloutSucceeded)
		event := finalizeTest(t, c)
		require.True(t, event.Complete)
		require.Equal(t, []string{"agent@sha256:new"}, event.Containers[0].RuntimeImageIDs)
	})
	t.Run("no scheduled Pods", func(t *testing.T) {
		d, cr := daemonSetFixture()
		d.Status = appsv1.DaemonSetStatus{ObservedGeneration: 1}
		c := testController(t)
		c.opts.client = fake.NewClientset()
		c.observeControllerRevision(cr)
		c.observeRevisioned(d, false, true)
		d.Generation++
		d.Status.ObservedGeneration = d.Generation
		c.observeRevisioned(d, false, false)
		c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
		takeDaemonSet(t, c, rolloutStarted)
		takeDaemonSet(t, c, rolloutSucceeded)
		require.False(t, finalizeTest(t, c).Complete)
	})
}

func TestDaemonSetDeletionClearsTrackingWithoutEvent(t *testing.T) {
	d, cr := daemonSetFixture()
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observeRevisioned(d, false, true)
	c.remove(cache.DeletedFinalStateUnknown{Obj: d})
	require.Empty(t, c.rollouts)
	require.Empty(t, c.workloads)
	require.Empty(t, c.revisions)
	require.Zero(t, c.queue.Len())
}

func TestWatcherTracksDaemonSets(t *testing.T) {
	d, cr := daemonSetFixture()
	client := fake.NewClientset(d, cr)
	received := make(chan plog.Logs, 8)
	c := newController(controllerOptions{clusterUID: "cluster-1", client: client, daemonSets: true, emit: func(_ context.Context, batch func() eventBatch) error { received <- batch().logs; return nil }})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.run(ctx) }()
	require.Eventually(t, func() bool {
		for _, action := range client.Actions() {
			if action.GetVerb() == "watch" && action.GetResource().Resource == "daemonsets" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	require.Empty(t, received, "initial observation is a silent baseline")
	_, err := client.AppsV1().ControllerRevisions(d.Namespace).Create(ctx, ownedRevision(d.UID, "DaemonSet", "web-b", 2), metav1.CreateOptions{})
	require.NoError(t, err)
	updateDaemonSet(d)
	d.Spec.Template.Spec.Containers[0].Image = "agent:2"
	_, err = client.AppsV1().DaemonSets(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	require.NoError(t, err)
	select {
	case logs := <-received:
		got := decode(t, logs)
		require.Contains(t, got.summary, "Rollout started")
		require.Equal(t, "daemonset", got.resource["grafana.sdlc.provider.resource.type"])
	case <-time.After(3 * time.Second):
		t.Fatal("no rollout event")
	}
	added, _ := daemonSetFixture()
	added.Name, added.UID = "new", "ds-2"
	_, err = client.AppsV1().DaemonSets(added.Namespace).Create(ctx, added, metav1.CreateOptions{})
	require.NoError(t, err)
	requireCreated(t, nextAdded(t, received), "daemonset")
	for _, action := range client.Actions() {
		require.NotEqual(t, "statefulsets", action.GetResource().Resource)
	}
	cancel()
	require.NoError(t, <-done)
}
