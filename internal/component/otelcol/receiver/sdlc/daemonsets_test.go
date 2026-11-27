package sdlc

import (
	"testing"

	"github.com/stretchr/testify/require"
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

func TestDaemonSetRolloutLifecycle(t *testing.T) {
	d, cr := daemonSetFixture()
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observe(d, observedInitial)
	c.observe(d, observedUpdated)
	// A new node schedules another Pod of the same revision.
	d.Status.DesiredNumberScheduled, d.Status.UpdatedNumberScheduled, d.Status.NumberAvailable = 3, 3, 3
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())

	// The workload update arrives before its ControllerRevision.
	updateDaemonSet(d)
	d.Spec.Template.Spec.Containers[0].Image = "agent:2"
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
	started := takeWorkload(t, c, "daemonset", rolloutStarted)
	require.Equal(t, "1", started.attrs["grafana.sdlc.change.version.before"])
	require.Equal(t, "2", started.attrs["grafana.sdlc.change.version.after"])
	require.Equal(t, "agent:1", started.image("before"))
	require.Equal(t, "agent:2", started.image("after"))
	require.Equal(t, "RollingUpdate", started.context["update_strategy"])
	require.NotContains(t, started.diff, "partition")

	completeDaemonSet(d)
	c.observe(d, observedUpdated)
	succeeded := takeWorkload(t, c, "daemonset", rolloutSucceeded)
	require.Equal(t, started.rolloutID, succeeded.rolloutID)
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
}

func TestDaemonSetSupersededStaleAndOnDelete(t *testing.T) {
	d, cr := daemonSetFixture()
	d.Spec.UpdateStrategy = appsv1.DaemonSetUpdateStrategy{Type: appsv1.OnDeleteDaemonSetStrategyType}
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observe(d, observedInitial)
	// The API server raises the generation before the controller writes the
	// new revision and observes the generation.
	updateDaemonSet(d)
	d.Status.ObservedGeneration--
	c.observe(d, observedUpdated)
	c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
	require.Zero(t, c.queue.Len())
	d.Status.ObservedGeneration = d.Generation
	c.observe(d, observedUpdated)
	require.Equal(t, "OnDelete", takeWorkload(t, c, "daemonset", rolloutStarted).context["update_strategy"])
	// The controller hasn't observed a newer generation yet.
	d.Generation++
	c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-c", 3))
	require.Zero(t, c.queue.Len())
	d.Status.ObservedGeneration = d.Generation
	c.observe(d, observedUpdated)
	require.EqualValues(t, 2, takeWorkload(t, c, "daemonset", rolloutSuperseded).revision)
	takeWorkload(t, c, "daemonset", rolloutStarted)
	// Pruning old revisions doesn't change the newest one.
	delete(c.revisions[string(d.UID)], "web-a")
	c.observe(d, observedUpdated)
	require.Zero(t, c.queue.Len())
	completeDaemonSet(d)
	c.observe(d, observedUpdated)
	takeWorkload(t, c, "daemonset", rolloutSucceeded)
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
		c.observe(d, observedInitial)
		c.observePod(old)
		updateDaemonSet(d)
		c.observe(d, observedUpdated)
		c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
		takeWorkload(t, c, "daemonset", rolloutStarted)
		c.observePod(node0)
		completeDaemonSet(d)
		c.observe(d, observedUpdated)
		takeWorkload(t, c, "daemonset", rolloutSucceeded)
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
		c.observe(d, observedInitial)
		d.Generation++
		d.Status.ObservedGeneration = d.Generation
		c.observe(d, observedUpdated)
		c.observeControllerRevision(ownedRevision(d.UID, "DaemonSet", "web-b", 2))
		takeWorkload(t, c, "daemonset", rolloutStarted)
		takeWorkload(t, c, "daemonset", rolloutSucceeded)
		require.False(t, finalizeTest(t, c).Complete)
	})
}

func TestDaemonSetDeletionClearsTrackingWithoutEvent(t *testing.T) {
	d, cr := daemonSetFixture()
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observe(d, observedInitial)
	c.remove(cache.DeletedFinalStateUnknown{Obj: d})
	require.Empty(t, c.rollouts)
	require.Empty(t, c.workloads)
	require.Empty(t, c.revisions)
	require.Zero(t, c.queue.Len())
}

func TestWatcherTracksDaemonSets(t *testing.T) {
	d, cr := daemonSetFixture()
	client := fake.NewClientset(d, cr)
	received := runWatcher(t, controllerOptions{clusterUID: "cluster-1", client: client, daemonSets: true})
	requireWatching(t, client.Actions, "daemonsets")
	require.Empty(t, received, "initial observation is a silent baseline")
	_, err := client.AppsV1().ControllerRevisions(d.Namespace).Create(t.Context(), ownedRevision(d.UID, "DaemonSet", "web-b", 2), metav1.CreateOptions{})
	require.NoError(t, err)
	updateDaemonSet(d)
	d.Spec.Template.Spec.Containers[0].Image = "agent:2"
	_, err = client.AppsV1().DaemonSets(d.Namespace).Update(t.Context(), d, metav1.UpdateOptions{})
	require.NoError(t, err)
	got := nextReceived(t, received)
	require.Contains(t, got.summary, "Rollout started")
	require.Equal(t, "daemonset", got.resource["grafana.sdlc.provider.resource.type"])
	added, _ := daemonSetFixture()
	added.Name, added.UID = "new", "ds-2"
	_, err = client.AppsV1().DaemonSets(added.Namespace).Create(t.Context(), added, metav1.CreateOptions{})
	require.NoError(t, err)
	requireCreated(t, nextAdded(t, received), "daemonset")
	for _, action := range client.Actions() {
		require.NotEqual(t, "statefulsets", action.GetResource().Resource)
	}
}
