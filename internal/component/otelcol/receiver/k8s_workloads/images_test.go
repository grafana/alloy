package k8s_workloads

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func imageFixtures() (*appsv1.Deployment, *appsv1.ReplicaSet, *corev1.Pod) {
	d := deploymentFixture()
	d.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	controller := true
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "web-rs", Namespace: d.Namespace, UID: "rs-2", Labels: map[string]string{"app": "web"}, Annotations: map[string]string{revisionAnnotation: "2"}, OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", UID: d.UID, Controller: &controller}}}}
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-pod", Namespace: d.Namespace, UID: "pod-1", Labels: map[string]string{"app": "web"}, OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", UID: rs.UID, Controller: &controller}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "app", ImageID: "registry/web@sha256:amd64"}}}}
	return d, rs, p
}
func finalizeTest(t *testing.T, c *controller) imagesEvent {
	t.Helper()
	require.Equal(t, 1, c.imageQueue.Len())
	x, _ := c.imageQueue.Get()
	c.imageQueue.Done(x)
	c.finalizeImages(t.Context(), x)
	batch, _ := c.queue.Get()
	c.queue.Done(batch)
	record := batch.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, eventPrefix+rolloutImagesResolved, record.EventName())
	var body imagesEvent
	require.NoError(t, json.Unmarshal([]byte(record.Body().Str()), &body))
	require.Equal(t, stableID(body.RolloutID, rolloutImagesResolved), batch.id)
	return body
}
func TestImagesFinalReadAndScalingSilence(t *testing.T) {
	d, rs, p := imageFixtures()
	c := testController(t)
	c.opts.client = fake.NewClientset(rs, p)
	c.observe(d, true)
	c.observeReplicaSet(rs)
	c.observePod(p)
	require.Empty(t, c.collections)
	require.Zero(t, c.imageQueue.Len())
	revision(d, 2)
	c.observe(d, false)
	take(t, c, "started")
	// Watch update has no image yet, but the final API read does.
	pending := p.DeepCopy()
	pending.Status.ContainerStatuses = nil
	c.observePod(pending)
	succeed(d)
	c.observe(d, false)
	take(t, c, "succeeded")
	event := finalizeTest(t, c)
	require.True(t, event.Complete)
	require.Equal(t, []string{"registry/web@sha256:amd64"}, event.Containers[0].RuntimeImageIDs)
	*d.Spec.Replicas = 2
	d.Generation++
	d.Status.ObservedGeneration = d.Generation
	c.observe(d, false)
	p.UID = "scaled-pod"
	p.Status.ContainerStatuses[0].ImageID = "registry/web@sha256:arm64"
	c.observePod(p)
	require.Empty(t, c.collections)
	require.Zero(t, c.imageQueue.Len())
	require.Zero(t, c.queue.Len())
}
func TestImagesAggregateDistinctIDsAndOwnerReferences(t *testing.T) {
	d, rs, p := imageFixtures()
	p2 := p.DeepCopy()
	p2.Name = "second"
	p2.UID = "pod-2"
	p2.Status.ContainerStatuses[0].ImageID = "registry/web@sha256:arm64"
	foreign := p.DeepCopy()
	foreign.Name = "foreign"
	foreign.UID = "pod-3"
	foreign.OwnerReferences[0].UID = types.UID("unrelated-rs")
	foreign.Status.ContainerStatuses[0].ImageID = "wrong"
	c := testController(t)
	c.opts.client = fake.NewClientset(rs, p, p2, foreign)
	c.observe(d, true)
	revision(d, 2)
	c.observe(d, false)
	take(t, c, "started")
	// Pod discovery before ReplicaSet discovery must still collect it.
	c.observePod(p)
	c.observePod(p2)
	c.observeReplicaSet(rs)
	c.observePod(p)
	c.observePod(foreign)
	succeed(d)
	c.observe(d, false)
	take(t, c, "succeeded")
	event := finalizeTest(t, c)
	require.True(t, event.Complete)
	require.Equal(t, []string{"registry/web@sha256:amd64", "registry/web@sha256:arm64"}, event.Containers[0].RuntimeImageIDs)
}
func TestImagesStalledThenSuperseded(t *testing.T) {
	d, rs, p := imageFixtures()
	c := testController(t)
	c.opts.client = fake.NewClientset(rs, p)
	c.observe(d, true)
	revision(d, 2)
	c.observe(d, false)
	take(t, c, "started")
	c.observeReplicaSet(rs)
	c.observePod(p)
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}
	c.observe(d, false)
	take(t, c, "stalled")
	require.Len(t, c.collections, 1)
	require.Zero(t, c.imageQueue.Len())
	revision(d, 3)
	c.observe(d, false)
	take(t, c, "superseded")
	take(t, c, "started")
	event := finalizeTest(t, c)
	require.False(t, event.Complete)
	require.EqualValues(t, 2, event.Revision)
	require.Equal(t, []string{"registry/web@sha256:amd64"}, event.Containers[0].RuntimeImageIDs)
	require.EqualValues(t, 3, c.collections[string(d.UID)].state.revision)
}
func TestImagesPartialResults(t *testing.T) {
	for _, mode := range []string{"missing image", "zero replicas", "list failure", "missing init image"} {
		t.Run(mode, func(t *testing.T) {
			d, rs, p := imageFixtures()
			c := testController(t)
			client := fake.NewClientset(rs, p)
			c.opts.client = client
			if mode == "missing image" {
				p.Status.ContainerStatuses = nil
				client = fake.NewClientset(rs, p)
				c.opts.client = client
			}
			if mode == "missing init image" {
				d.Spec.Template.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "setup:v1"}}
			}
			c.observe(d, true)
			revision(d, 2)
			c.observe(d, false)
			take(t, c, "started")
			c.observeReplicaSet(rs)
			c.observePod(p)
			if mode == "zero replicas" {
				*d.Spec.Replicas = 0
			}
			if mode == "list failure" {
				client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("unavailable") })
			}
			succeed(d)
			c.observe(d, false)
			take(t, c, "succeeded")
			event := finalizeTest(t, c)
			require.False(t, event.Complete)
			require.Zero(t, c.imageQueue.Len())
		})
	}
}
