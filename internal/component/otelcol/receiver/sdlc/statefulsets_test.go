package sdlc

import (
	"context"
	"strconv"
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

func statefulSetFixture() (*appsv1.StatefulSet, *appsv1.ControllerRevision) {
	s := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "demo", UID: "s-1", Generation: 1}, Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(2)), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "postgres:16"}}}}}, Status: appsv1.StatefulSetStatus{ObservedGeneration: 1, Replicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2, CurrentRevision: "web-1", UpdateRevision: "web-1"}}
	return s, ownedRevision(s.UID, "web-1", 1)
}

func ownedRevision(owner types.UID, name string, n int64) *appsv1.ControllerRevision {
	return &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "demo", UID: types.UID("cr-" + name), Labels: map[string]string{controllerRevisionHashLabel: name[len("web-"):]}, OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", UID: owner, Controller: ptr.To(true)}}}, Revision: n}
}

// updateStatefulSet starts rolling out a new template as revision name.
func updateStatefulSet(s *appsv1.StatefulSet, name string) {
	s.Generation++
	s.Status.ObservedGeneration = s.Generation
	s.Status.UpdateRevision = name
	s.Status.UpdatedReplicas = 0
	s.Status.AvailableReplicas = *s.Spec.Replicas - 1
}

func completeStatefulSet(s *appsv1.StatefulSet) {
	s.Status.UpdatedReplicas = *s.Spec.Replicas
	s.Status.AvailableReplicas = *s.Spec.Replicas
	s.Status.CurrentRevision = s.Status.UpdateRevision
}

func takeStatefulSet(t *testing.T, c *controller, status string) decoded {
	t.Helper()
	_, got := takeWorkload(t, c, status, "statefulset")
	return got
}

func TestStatefulSetRolloutLifecycle(t *testing.T) {
	s, cr := statefulSetFixture()
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observeRevisioned(s, true)
	c.observeRevisioned(s, false)
	// Scaling changes generation, not the update revision.
	s.Generation++
	s.Status.ObservedGeneration = s.Generation
	s.Spec.Replicas = ptr.To(int32(3))
	s.Status.Replicas, s.Status.UpdatedReplicas, s.Status.AvailableReplicas = 3, 3, 3
	c.observeRevisioned(s, false)
	require.Zero(t, c.queue.Len())

	// The workload update arrives before its ControllerRevision.
	updateStatefulSet(s, "web-2")
	s.Spec.Template.Spec.Containers[0].Image = "postgres:17"
	c.observeRevisioned(s, false)
	require.Zero(t, c.queue.Len())
	c.observeControllerRevision(ownedRevision(s.UID, "web-2", 2))
	started := takeStatefulSet(t, c, rolloutStarted)
	require.Equal(t, int64(2), started.revision)
	require.Equal(t, "1", started.attrs["grafana.sdlc.change.version.before"])
	require.Equal(t, "2", started.attrs["grafana.sdlc.change.version.after"])
	require.Equal(t, map[string]any{"operation": "modified", "before": "postgres:16", "after": "postgres:17"}, started.changes()["containers.app.image"])
	require.Equal(t, "RollingUpdate", started.diff["update_strategy"])
	require.Equal(t, int64(0), started.diff["partition"])

	// All Pods updated, but the controller hasn't promoted the revision yet.
	s.Status.UpdatedReplicas, s.Status.AvailableReplicas = 3, 3
	c.observeRevisioned(s, false)
	require.Zero(t, c.queue.Len())
	completeStatefulSet(s)
	c.observeRevisioned(s, false)
	succeeded := takeStatefulSet(t, c, rolloutSucceeded)
	require.Equal(t, started.rolloutID, succeeded.rolloutID)
	c.observeRevisioned(s, false)
	require.Zero(t, c.queue.Len())
}

func TestStatefulSetUpdateStrategies(t *testing.T) {
	t.Run("partition", func(t *testing.T) {
		s, cr := statefulSetFixture()
		s.Spec.Replicas = ptr.To(int32(3))
		s.Status.Replicas, s.Status.UpdatedReplicas, s.Status.AvailableReplicas = 3, 3, 3
		s.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType, RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{Partition: ptr.To(int32(1))}}
		c := testController(t)
		c.observeControllerRevision(cr)
		c.observeControllerRevision(ownedRevision(s.UID, "web-2", 2))
		c.observeRevisioned(s, true)
		updateStatefulSet(s, "web-2")
		c.observeRevisioned(s, false)
		started := takeStatefulSet(t, c, rolloutStarted)
		require.Equal(t, int64(1), started.diff["partition"])
		s.Status.UpdatedReplicas = 1
		c.observeRevisioned(s, false)
		require.Zero(t, c.queue.Len())
		// Ordinals 1 and 2 are updated; ordinal 0 keeps the current revision.
		s.Status.UpdatedReplicas, s.Status.AvailableReplicas = 2, 3
		c.observeRevisioned(s, false)
		takeStatefulSet(t, c, rolloutSucceeded)
	})
	t.Run("OnDelete", func(t *testing.T) {
		s, cr := statefulSetFixture()
		s.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.OnDeleteStatefulSetStrategyType}
		c := testController(t)
		c.observeControllerRevision(cr)
		c.observeControllerRevision(ownedRevision(s.UID, "web-2", 2))
		c.observeRevisioned(s, true)
		updateStatefulSet(s, "web-2")
		c.observeRevisioned(s, false)
		require.Equal(t, "OnDelete", takeStatefulSet(t, c, rolloutStarted).diff["update_strategy"])
		// OnDelete never promotes the current revision.
		s.Status.UpdatedReplicas, s.Status.AvailableReplicas = 2, 2
		c.observeRevisioned(s, false)
		takeStatefulSet(t, c, rolloutSucceeded)
	})
}

func TestStatefulSetSupersededStaleAndRollback(t *testing.T) {
	s, cr := statefulSetFixture()
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observeRevisioned(s, true)
	c.observeControllerRevision(ownedRevision(s.UID, "web-2", 2))
	updateStatefulSet(s, "web-2")
	c.observeRevisioned(s, false)
	takeStatefulSet(t, c, rolloutStarted)
	stale := s.DeepCopy()
	c.observeControllerRevision(ownedRevision(s.UID, "web-3", 3))
	updateStatefulSet(s, "web-3")
	c.observeRevisioned(s, false)
	superseded := takeStatefulSet(t, c, rolloutSuperseded)
	require.Equal(t, int64(2), superseded.revision)
	takeStatefulSet(t, c, rolloutStarted)
	c.observeRevisioned(stale, false)
	require.Zero(t, c.queue.Len())

	// A rollback reuses the original ControllerRevision with a new number.
	completeStatefulSet(s)
	c.observeRevisioned(s, false)
	takeStatefulSet(t, c, rolloutSucceeded)
	updateStatefulSet(s, "web-1")
	c.observeRevisioned(s, false)
	require.Zero(t, c.queue.Len(), "web-1 is still revision 1 until the controller renumbers it")
	c.observeControllerRevision(ownedRevision(s.UID, "web-1", 4))
	rollback := takeStatefulSet(t, c, rolloutStarted)
	require.Equal(t, "3", rollback.attrs["grafana.sdlc.change.version.before"])
	require.Equal(t, "4", rollback.attrs["grafana.sdlc.change.version.after"])
}

func TestStatefulSetImagesMatchRevisionHash(t *testing.T) {
	s, cr := statefulSetFixture()
	pod := func(name, hash, image string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.Namespace, UID: types.UID(name), Labels: map[string]string{"app": "web", controllerRevisionHashLabel: hash}, OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", UID: s.UID, Controller: ptr.To(true)}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "app", ImageID: image}}}}
	}
	updated0, updated1, old := pod("web-0", "web-2", "postgres@sha256:new"), pod("web-1", "web-2", "postgres@sha256:new"), pod("web-old", "web-1", "postgres@sha256:old")
	c := testController(t)
	c.opts.client = fake.NewClientset(updated0, updated1, old)
	c.observeControllerRevision(cr)
	c.observeRevisioned(s, true)
	c.observePod(old)
	c.observeControllerRevision(ownedRevision(s.UID, "web-2", 2))
	updateStatefulSet(s, "web-2")
	c.observeRevisioned(s, false)
	takeStatefulSet(t, c, rolloutStarted)
	c.observePod(updated0)
	completeStatefulSet(s)
	c.observeRevisioned(s, false)
	takeStatefulSet(t, c, rolloutSucceeded)
	event := finalizeTest(t, c)
	require.True(t, event.Complete)
	require.EqualValues(t, 2, event.Revision)
	require.Equal(t, []string{"postgres@sha256:new"}, event.Containers[0].RuntimeImageIDs)
}

func TestStatefulSetDeletionClearsTrackingWithoutEvent(t *testing.T) {
	s, cr := statefulSetFixture()
	c := testController(t)
	c.observeControllerRevision(cr)
	c.observeRevisioned(s, true)
	c.remove(cache.DeletedFinalStateUnknown{Obj: s})
	require.Empty(t, c.rollouts)
	require.Empty(t, c.workloads)
	require.Empty(t, c.revisions)
	require.Zero(t, c.queue.Len())
}

func TestControllerRevisionTransformDropsData(t *testing.T) {
	_, cr := statefulSetFixture()
	cr.Labels["unused"] = "large"
	cr.Data.Raw = []byte(`{"spec":{"template":{}}}`)
	transformed, err := imageWatchTransform(cr)
	require.NoError(t, err)
	got := transformed.(*appsv1.ControllerRevision)
	require.Equal(t, map[string]string{controllerRevisionHashLabel: "1"}, got.Labels)
	require.Equal(t, cr.OwnerReferences, got.OwnerReferences)
	require.EqualValues(t, 1, got.Revision)
	require.Empty(t, got.Data.Raw)
	again, err := imageWatchTransform(got)
	require.NoError(t, err)
	require.Equal(t, got, again)
}

func TestWatcherTracksStatefulSets(t *testing.T) {
	s, cr := statefulSetFixture()
	client := fake.NewClientset(s, cr)
	received := make(chan plog.Logs, 8)
	c := newController(controllerOptions{clusterUID: "cluster-1", client: client, statefulSets: true, emit: func(_ context.Context, batch func() eventBatch) error { received <- batch().logs; return nil }})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.run(ctx) }()
	require.Eventually(t, func() bool {
		for _, action := range client.Actions() {
			if action.GetVerb() == "watch" && action.GetResource().Resource == "statefulsets" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	require.Empty(t, received, "initial observation is a silent baseline")
	next := ownedRevision(s.UID, "web-2", 2)
	_, err := client.AppsV1().ControllerRevisions(s.Namespace).Create(ctx, next, metav1.CreateOptions{})
	require.NoError(t, err)
	updateStatefulSet(s, "web-2")
	s.Spec.Template.Spec.Containers[0].Image = "postgres:17"
	_, err = client.AppsV1().StatefulSets(s.Namespace).Update(ctx, s, metav1.UpdateOptions{})
	require.NoError(t, err)
	select {
	case logs := <-received:
		got := decode(t, logs)
		require.Contains(t, got.summary, "Rollout started")
		require.Equal(t, "statefulset", got.resource["grafana.sdlc.resource.type"])
		require.Equal(t, strconv.Itoa(2), got.attrs["grafana.sdlc.change.version.after"])
	case <-time.After(3 * time.Second):
		t.Fatal("no rollout event")
	}
	cancel()
	require.NoError(t, <-done)
}
