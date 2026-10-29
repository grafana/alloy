package sdlc

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// workload is a controller whose pod template changes roll out as revisions.
// The rollout state machine and image resolution are shared across kinds.
type workload interface {
	metav1.Object
	resourceType() string
	template() corev1.PodTemplateSpec
	selector() *metav1.LabelSelector
	// revision is the controller-assigned revision, or 0 if not yet known.
	revision() int64
	// revisionHash is the controller-revision-hash label of the revision's Pods.
	// It is empty for Deployments, whose Pods are matched through ReplicaSets.
	revisionHash() string
	// settled reports whether status describes the current spec: the controller
	// has observed it, and the workload is neither paused nor being deleted.
	settled() bool
	rolloutStatus() string
	// desiredPods is how many Pods run the revision once it has rolled out.
	desiredPods() int32
	// putDiff adds kind-specific keys to a rollout event's change.diff.
	putDiff(pcommon.Map)
	copy() workload
}

type deploymentWorkload struct{ *appsv1.Deployment }

func (d deploymentWorkload) resourceType() string             { return "deployment" }
func (d deploymentWorkload) template() corev1.PodTemplateSpec { return d.Spec.Template }
func (d deploymentWorkload) selector() *metav1.LabelSelector  { return d.Spec.Selector }
func (d deploymentWorkload) revisionHash() string             { return "" }
func (d deploymentWorkload) putDiff(pcommon.Map)              {}
func (d deploymentWorkload) copy() workload                   { return deploymentWorkload{d.DeepCopy()} }
func (d deploymentWorkload) desiredPods() int32               { return replicas(d.Spec.Replicas) }

// Revisions are assigned by the Deployment controller, unlike generation which
// also changes during scaling.
func (d deploymentWorkload) revision() int64 {
	revision, _ := strconv.ParseInt(d.Annotations[revisionAnnotation], 10, 64)
	return revision
}

func (d deploymentWorkload) settled() bool {
	return !d.Spec.Paused && d.DeletionTimestamp == nil && d.Status.ObservedGeneration >= d.Generation
}

func (d deploymentWorkload) rolloutStatus() string {
	if !d.settled() {
		return rolloutStarted
	}
	for _, condition := range d.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded" {
			return rolloutStalled
		}
	}
	desired := d.desiredPods()
	// Require the controller's completion condition as well as replica counts.
	for _, condition := range d.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionTrue && condition.Reason == "NewReplicaSetAvailable" && d.Status.UpdatedReplicas == desired && d.Status.Replicas == desired && d.Status.AvailableReplicas == desired && d.Status.UnavailableReplicas == 0 {
			return rolloutSucceeded
		}
	}
	return rolloutStarted
}

// replicas defaults an unset replica count to 1, as the API server does.
func replicas(n *int32) int32 {
	if n == nil {
		return 1
	}
	return *n
}
