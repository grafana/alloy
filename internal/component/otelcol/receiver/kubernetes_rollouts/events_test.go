package kubernetes_rollouts

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestDeploymentPhase(t *testing.T) {
	replicas := int32(2)
	base := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	}

	tests := []struct {
		name   string
		mutate func(*appsv1.Deployment)
		want   rolloutPhase
	}{
		{name: "started", mutate: func(*appsv1.Deployment) {}, want: phaseStarted},
		{name: "succeeded", mutate: func(d *appsv1.Deployment) {
			d.Status.ObservedGeneration = 3
			d.Status.UpdatedReplicas = 2
			d.Status.Replicas = 2
			d.Status.AvailableReplicas = 2
		}, want: phaseSucceeded},
		{name: "stalled", mutate: func(d *appsv1.Deployment) {
			d.Status.Conditions = []appsv1.DeploymentCondition{{
				Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded",
			}}
		}, want: phaseStalled},
		{name: "replica failure", mutate: func(d *appsv1.Deployment) {
			d.Status.Conditions = []appsv1.DeploymentCondition{{
				Type: appsv1.DeploymentReplicaFailure, Status: corev1.ConditionTrue, Reason: "FailedCreate",
			}}
		}, want: phaseStalled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := base.DeepCopy()
			tt.mutate(deployment)
			require.Equal(t, tt.want, deploymentPhase(deployment))
		})
	}
}

func TestImageMetadata(t *testing.T) {
	image := newImageData("api", "registry.example.com/team/api:1.2.5", false)
	require.Equal(t, "registry.example.com/team/api", image.imageName)
	require.Equal(t, "1.2.5", image.tag)
	require.Empty(t, image.digest)

	pinned := newImageData("api", "registry.example.com/team/api:1.2.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false)
	require.Equal(t, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", pinned.digest)

	require.Equal(t, "sha256:bbbb", imageDigest("containerd://registry.example.com/team/api@sha256:bbbb"))
	require.Equal(t, "sha256:cccc", imageDigest("containerd://sha256:cccc"))
	require.Empty(t, imageDigest("containerd://opaque-id"))
}

func TestCollectImagesPreservesDistinctDigests(t *testing.T) {
	deployment := testDeployment()
	rs := testReplicaSet(deployment)
	firstPod := testPod(rs)
	secondPod := testPod(rs)
	secondPod.Name = "checkout-abc-2"
	secondPod.Status.ContainerStatuses[0].ImageID = "containerd://sha256:cccc"
	podIndexer := namespacedIndexer()
	require.NoError(t, podIndexer.Add(firstPod))
	require.NoError(t, podIndexer.Add(secondPod))

	images := collectImages(deployment.Spec.Template.Spec, rs, corelisters.NewPodLister(podIndexer))
	require.Len(t, images, 2)
	require.Equal(t, "sha256:cccc", images[0].digest)
	require.Equal(t, "sha256:bbbb", images[1].digest)
}

func TestPodSpecsUseSameImages(t *testing.T) {
	left := corev1.PodSpec{
		Containers:     []corev1.Container{{Name: "api", Image: "example/api:1"}},
		InitContainers: []corev1.Container{{Name: "migrate", Image: "example/migrate:1"}},
	}
	right := corev1.PodSpec{
		Containers:     []corev1.Container{{Name: "api", Image: "example/api:1"}},
		InitContainers: []corev1.Container{{Name: "migrate", Image: "example/migrate:1"}},
	}
	require.True(t, podSpecsUseSameImages(left, right))

	right.Containers[0].Image = "example/api:2"
	require.False(t, podSpecsUseSameImages(left, right))
}

func TestBuildDeploymentScopedEventBatch(t *testing.T) {
	deployment := testDeployment()
	rs := testReplicaSet(deployment)
	image := imageData{
		container: "api", reference: "registry.example.com/team/api:1.2.5",
		imageName: "registry.example.com/team/api", tag: "1.2.5",
		imageID: "registry.example.com/team/api@sha256:bbbb", digest: "sha256:bbbb",
	}
	sidecar := imageData{
		container: "sidecar", reference: "registry.example.com/team/sidecar:2.0.0",
		imageName: "registry.example.com/team/sidecar", tag: "2.0.0",
	}
	data := eventData{
		clusterUID: "cluster-uid", clusterName: "production", deployment: deployment,
		replicaSet: rs, revision: "7", phase: phaseStarted, images: []imageData{image, sidecar},
	}

	first := buildEventBatch(data).logs
	second := buildEventBatch(data).logs
	firstResource := first.ResourceLogs().At(0).Resource().Attributes()
	requireAttributeString(t, firstResource, "k8s.cluster.uid", "cluster-uid")
	requireAttributeString(t, firstResource, "k8s.cluster.name", "production")
	requireAttributeString(t, firstResource, "k8s.namespace.name", "payments")
	requireAttributeString(t, firstResource, "k8s.deployment.name", "checkout")

	records := first.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 1, records.Len())
	record := records.At(0)
	require.Equal(t, "grafana.sdlc.k8s.deployment.rollout.started", record.EventName())
	containersValue, ok := record.Attributes().Get("grafana.sdlc.deployment.containers")
	require.True(t, ok)
	require.Len(t, containersValue.Slice().AsRaw(), 2)
	container := containersValue.Slice().At(0).Map()
	requireAttributeString(t, container, "name", "api")
	requireAttributeString(t, container, "image.name", "registry.example.com/team/api")
	requireAttributeString(t, container, "image.id", image.imageID)
	requireAttributeString(t, container, "image.reference", image.reference)

	firstID, _ := record.Attributes().Get("grafana.sdlc.event.id")
	secondID, _ := second.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("grafana.sdlc.event.id")
	require.Equal(t, firstID.Str(), secondID.Str())
}

func TestBuildImageResolvedEventBatch(t *testing.T) {
	deployment := testDeployment()
	images := []imageData{
		{
			container: "api", reference: "registry.example.com/team/api:1.2.5",
			imageName: "registry.example.com/team/api", tag: "1.2.5", digest: "sha256:aaaa",
		},
		{
			container: "sidecar", reference: "registry.example.com/team/sidecar:2.0.0",
			imageName: "registry.example.com/team/sidecar", tag: "2.0.0", digest: "sha256:bbbb",
		},
	}
	data := eventData{
		clusterUID: "cluster-uid", deployment: deployment, revision: "7",
		phase: phaseImageResolved, images: images,
	}

	records := buildEventBatch(data).logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 2, records.Len())
	for i, image := range images {
		record := records.At(i)
		require.Equal(t, "grafana.sdlc.k8s.deployment.rollout.image_resolved", record.EventName())
		requireAttributeString(t, record.Attributes(), "k8s.container.name", image.container)
		requireAttributeString(t, record.Attributes(), "container.image.name", image.imageName)
		_, ok := record.Attributes().Get("grafana.sdlc.deployment.containers")
		require.False(t, ok)
	}
}

func TestBuildObservedInventoryEvent(t *testing.T) {
	deployment := testDeployment()
	deployment.Labels = map[string]string{"app.kubernetes.io/name": "checkout", "team": "payments"}
	deployment.Status.UpdatedReplicas = 1
	image := imageData{
		container: "api", reference: "registry.example.com/team/api:1.2.5",
		imageName: "registry.example.com/team/api", tag: "1.2.5",
		imageID: "registry.example.com/team/api@sha256:bbbb", digest: "sha256:bbbb",
	}
	data := eventData{
		clusterUID: "cluster-uid", clusterName: "production", deployment: deployment,
		revision: "7", phase: phaseStarted, images: []imageData{image},
	}
	fingerprint := inventoryFingerprint(data)
	first := buildInventoryEventBatch(data, inventoryObserved, fingerprint).logs
	second := buildInventoryEventBatch(data, inventoryObserved, fingerprint).logs
	record := first.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "grafana.sdlc.k8s.deployment.observed", record.EventName())
	requireAttributeString(t, record.Attributes(), "grafana.sdlc.inventory.version", fingerprint)
	labels, ok := record.Attributes().Get("grafana.sdlc.k8s.deployment.labels")
	require.True(t, ok)
	requireAttributeString(t, labels.Map(), "team", "payments")

	containerValue, ok := record.Attributes().Get("grafana.sdlc.deployment.containers")
	require.True(t, ok)
	containers := containerValue.Slice()
	require.Len(t, containers.AsRaw(), 1)
	container := containers.At(0).Map()
	requireAttributeString(t, container, "name", "api")
	requireAttributeString(t, container, "image.reference", image.reference)
	requireAttributeString(t, container, "image.name", image.imageName)
	repoDigests, ok := container.Get("image.repo_digests")
	require.True(t, ok)
	require.Equal(t, image.imageName+"@"+image.digest, repoDigests.Slice().At(0).Str())

	firstID, _ := record.Attributes().Get("grafana.sdlc.event.id")
	secondRecord := second.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	secondID, _ := secondRecord.Attributes().Get("grafana.sdlc.event.id")
	require.Equal(t, firstID.Str(), secondID.Str())
}

func TestReconcileLifecycleAndRetry(t *testing.T) {
	deployment := testDeployment()
	rs := testReplicaSet(deployment)
	pod := testPod(rs)

	deploymentIndexer := namespacedIndexer()
	replicaSetIndexer := namespacedIndexer()
	podIndexer := namespacedIndexer()
	require.NoError(t, deploymentIndexer.Add(deployment))
	require.NoError(t, replicaSetIndexer.Add(rs))
	require.NoError(t, podIndexer.Add(pod))

	var eventNames []string
	failNext := true
	ctrl := newController(controllerOptions{
		clusterName: "production",
		emit: func(_ context.Context, batch func() eventBatch) error {
			if failNext {
				failNext = false
				return errors.New("export unavailable")
			}
			records := batch().logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
			for i := 0; i < records.Len(); i++ {
				eventNames = append(eventNames, records.At(i).EventName())
			}
			return nil
		},
	})
	ctrl.deployments = appslisters.NewDeploymentLister(deploymentIndexer)
	ctrl.replicaSets = appslisters.NewReplicaSetLister(replicaSetIndexer)
	ctrl.pods = corelisters.NewPodLister(podIndexer)
	images := collectImages(deployment.Spec.Template.Spec, rs, ctrl.pods)
	require.Len(t, images, 1)
	require.Equal(t, "sha256:bbbb", images[0].digest)

	err := ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout")
	require.EqualError(t, err, "export unavailable")

	require.NoError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"))
	require.Equal(t, []string{
		"grafana.sdlc.k8s.deployment.rollout.started",
		"grafana.sdlc.k8s.deployment.rollout.image_resolved",
		"grafana.sdlc.k8s.deployment.observed",
	}, eventNames)

	eventNames = nil
	require.NoError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"))
	require.Empty(t, eventNames)

	deployment.Status.ObservedGeneration = deployment.Generation
	deployment.Status.UpdatedReplicas = 1
	deployment.Status.Replicas = 1
	deployment.Status.AvailableReplicas = 1
	require.NoError(t, deploymentIndexer.Update(deployment))
	require.NoError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"))
	require.Equal(t, []string{
		"grafana.sdlc.k8s.deployment.rollout.succeeded",
		"grafana.sdlc.k8s.deployment.observed",
	}, eventNames)
}

func TestReconcileRevisionUpdateDoesNotDuplicateStart(t *testing.T) {
	deployment := testDeployment()
	rs := testReplicaSet(deployment)

	deploymentIndexer := namespacedIndexer()
	replicaSetIndexer := namespacedIndexer()
	podIndexer := namespacedIndexer()
	require.NoError(t, deploymentIndexer.Add(deployment))
	require.NoError(t, replicaSetIndexer.Add(rs))

	var eventNames []string
	ctrl := newController(controllerOptions{emit: func(_ context.Context, batch func() eventBatch) error {
		records := batch().logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
		for i := 0; i < records.Len(); i++ {
			eventNames = append(eventNames, records.At(i).EventName())
		}
		return nil
	}})
	ctrl.deployments = appslisters.NewDeploymentLister(deploymentIndexer)
	ctrl.replicaSets = appslisters.NewReplicaSetLister(replicaSetIndexer)
	ctrl.pods = corelisters.NewPodLister(podIndexer)

	require.NoError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"))
	eventNames = nil

	deployment.Annotations[deploymentRevisionAnnotation] = "8"
	require.NoError(t, deploymentIndexer.Update(deployment))
	rs.Annotations[deploymentRevisionAnnotation] = "8"
	require.NoError(t, replicaSetIndexer.Update(rs))
	require.NoError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"))
	require.Equal(t, []string{"grafana.sdlc.k8s.deployment.observed"}, eventNames)
}

func TestReconcileForgetsDeletedDeployment(t *testing.T) {
	deployment := testDeployment()
	rs := testReplicaSet(deployment)

	deploymentIndexer := namespacedIndexer()
	replicaSetIndexer := namespacedIndexer()
	podIndexer := namespacedIndexer()
	require.NoError(t, deploymentIndexer.Add(deployment))
	require.NoError(t, replicaSetIndexer.Add(rs))

	var eventNames []string
	failDelete := true
	ctrl := newController(controllerOptions{emit: func(_ context.Context, batch func() eventBatch) error {
		records := batch().logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
		for i := 0; i < records.Len(); i++ {
			name := records.At(i).EventName()
			if name == "grafana.sdlc.k8s.deployment.deleted" && failDelete {
				failDelete = false
				return errors.New("export unavailable")
			}
			eventNames = append(eventNames, name)
		}
		return nil
	}})
	ctrl.deployments = appslisters.NewDeploymentLister(deploymentIndexer)
	ctrl.replicaSets = appslisters.NewReplicaSetLister(replicaSetIndexer)
	ctrl.pods = corelisters.NewPodLister(podIndexer)

	require.NoError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"))
	require.Contains(t, ctrl.states, deployment.UID)
	require.NoError(t, deploymentIndexer.Delete(deployment))
	require.EqualError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"), "export unavailable")
	require.Contains(t, ctrl.states, deployment.UID)
	require.Contains(t, ctrl.keys, "payments/checkout")
	require.NoError(t, ctrl.reconcile(t.Context(), "cluster-uid", "payments/checkout"))
	require.Contains(t, eventNames, "grafana.sdlc.k8s.deployment.deleted")
	require.Empty(t, ctrl.states)
	require.Empty(t, ctrl.keys)
}

func testDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "checkout", Namespace: "payments", UID: types.UID("deployment-uid"), Generation: 5,
			Annotations: map[string]string{deploymentRevisionAnnotation: "7"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "api", Image: "registry.example.com/team/api:1.2.5",
			}}}},
		},
	}
}

func namespacedIndexer() cache.Indexer {
	return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
}

func testReplicaSet(deployment *appsv1.Deployment) *appsv1.ReplicaSet {
	controller := true
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "checkout-abc", Namespace: deployment.Namespace, UID: types.UID("replicaset-uid"),
			Annotations: map[string]string{deploymentRevisionAnnotation: "7"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name, UID: deployment.UID, Controller: &controller,
			}},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "checkout"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "checkout"}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "api", Image: "registry.example.com/team/api:1.2.5",
				}}},
			},
		},
	}
}

func testPod(rs *appsv1.ReplicaSet) *corev1.Pod {
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "checkout-abc-1", Namespace: rs.Namespace, Labels: map[string]string{"app": "checkout"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1", Kind: "ReplicaSet", Name: rs.Name, UID: rs.UID, Controller: &controller,
			}},
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: "api", Image: "registry.example.com/team/api:1.2.5",
			ImageID: "registry.example.com/team/api@sha256:bbbb",
		}}},
	}
}

func requireAttributeString(t *testing.T, attrs pcommon.Map, key, expected string) {
	t.Helper()
	value, ok := attrs.Get(key)
	require.True(t, ok)
	require.Equal(t, expected, value.Str())
}
