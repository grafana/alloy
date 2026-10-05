package k8s_workloads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
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

func TestImageWatchTransformDropsUnusedFields(t *testing.T) {
	owner := metav1.OwnerReference{Kind: "ReplicaSet", Name: "rs", UID: "rs-uid", Controller: ptr.To(true)}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns", UID: "pod-uid", ResourceVersion: "123", Annotations: map[string]string{"large": "unused"}, OwnerReferences: []metav1.OwnerReference{owner}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "example/app", Env: []corev1.EnvVar{{Name: "UNUSED", Value: "large"}}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "app", ImageID: "runtime://digest", ContainerID: "unneeded"}}}}
	transformed, err := imageWatchTransform(pod)
	require.NoError(t, err)
	got := transformed.(*corev1.Pod)
	require.Equal(t, pod.UID, got.UID)
	require.Equal(t, pod.ResourceVersion, got.ResourceVersion)
	require.Equal(t, pod.OwnerReferences, got.OwnerReferences)
	require.Equal(t, corev1.PodRunning, got.Status.Phase)
	require.Equal(t, []corev1.ContainerStatus{{Name: "app", ImageID: "runtime://digest"}}, got.Status.ContainerStatuses)
	require.Empty(t, got.Annotations)
	require.Empty(t, got.Spec.Containers)
	require.NotEmpty(t, pod.Spec.Containers, "transform must not mutate input")
	rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{UID: "rs-uid", Annotations: map[string]string{revisionAnnotation: "2", "unused": "large"}}, Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{Spec: pod.Spec}}}
	transformed, err = imageWatchTransform(rs)
	require.NoError(t, err)
	gotRS := transformed.(*appsv1.ReplicaSet)
	require.Equal(t, map[string]string{revisionAnnotation: "2"}, gotRS.Annotations)
	require.Empty(t, gotRS.Spec.Template.Spec.Containers)
	again, err := imageWatchTransform(got)
	require.NoError(t, err)
	require.Equal(t, got, again, "informer transforms may run repeatedly")
}

func TestImageListPagesBeforeRetainingObjects(t *testing.T) {
	calls := 0
	result, err := imageList(context.Background(), metav1.ListOptions{ResourceVersion: "0"}, func(_ context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		calls++
		require.Equal(t, int64(250), opts.Limit)
		require.Empty(t, opts.ResourceVersion)
		next := ""
		if calls == 1 {
			require.Empty(t, opts.Continue)
			next = "page-2"
		} else {
			require.Equal(t, "page-2", opts.Continue)
		}
		return &appsv1.ReplicaSetList{ListMeta: metav1.ListMeta{ResourceVersion: "123", Continue: next}, Items: []appsv1.ReplicaSet{{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprint(calls), Annotations: map[string]string{revisionAnnotation: "2"}}, Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "unused", Image: "large"}}}}}}}}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	items, err := meta.ExtractList(result)
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.Empty(t, item.(*appsv1.ReplicaSet).Spec.Template.Spec.Containers)
	}
	metadata, err := meta.ListAccessor(result)
	require.NoError(t, err)
	require.Equal(t, "123", metadata.GetResourceVersion())
	require.Empty(t, metadata.GetContinue())
	calls = 0
	_, err = imageList(context.Background(), metav1.ListOptions{}, func(_ context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		calls++
		require.Equal(t, int64(250), opts.Limit)
		if calls == 1 {
			return &corev1.PodList{ListMeta: metav1.ListMeta{Continue: "expired"}}, nil
		}
		return nil, apierrors.NewResourceExpired("expired")
	})
	require.True(t, apierrors.IsResourceExpired(err))
	require.Equal(t, 2, calls, "must not fall back to unbounded list")
}

func TestImageOwnerIndexesFollowUpdatesAndDeletion(t *testing.T) {
	d, rs, p := imageFixtures()
	c := testController(t)
	c.observePod(p)
	c.observeReplicaSet(rs)
	c.openImages(d, rolloutState{revision: 2, template: d.Spec.Template})
	require.True(t, c.collections[string(d.UID)].ids["false:app"]["registry/web@sha256:amd64"])
	moved := p.DeepCopy()
	moved.OwnerReferences[0].UID = "other-rs"
	c.observePod(moved)
	require.Empty(t, c.podsByReplicaSet[string(rs.UID)])
	require.Len(t, c.podsByReplicaSet["other-rs"], 1)
	c.removePod(moved)
	require.Empty(t, c.podsByReplicaSet)
	movedRS := rs.DeepCopy()
	movedRS.OwnerReferences[0].UID = "other-deployment"
	c.observeReplicaSet(movedRS)
	require.Empty(t, c.replicaSetsByDeployment[string(d.UID)])
	require.Len(t, c.replicaSetsByDeployment["other-deployment"], 1)
	c.removeReplicaSet(movedRS)
	require.Empty(t, c.replicaSetsByDeployment)
}

func BenchmarkReplicaSetUpdateWithUnrelatedWorkloads(b *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			c := newController(controllerOptions{})
			defer c.queue.ShutDown()
			defer c.imageQueue.ShutDown()
			for i := 0; i < count; i++ {
				d, rs, p := imageFixtures()
				d.UID = types.UID(fmt.Sprint("deployment-", i))
				rs.UID = types.UID(fmt.Sprint("rs-", i))
				rs.OwnerReferences[0].UID = d.UID
				p.UID = types.UID(fmt.Sprint("pod-", i))
				p.OwnerReferences[0].UID = rs.UID
				c.observePod(p)
				c.observeReplicaSet(rs)
				c.openImages(d, rolloutState{revision: 2, template: d.Spec.Template})
			}
			_, rs, _ := imageFixtures()
			b.ResetTimer()
			for b.Loop() {
				c.observeReplicaSet(rs)
			}
		})
	}
}
