package sdlc

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetsArguments configures StatefulSet rollout tracking.
type StatefulSetsArguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *StatefulSetsArguments) SetToDefault() {
	*args = StatefulSetsArguments{Enabled: true}
}

// statefulSetWorkload is a StatefulSet with its update revision resolved from
// the ControllerRevision index; current.revision is 0 until it's known.
type statefulSetWorkload struct {
	*appsv1.StatefulSet
	current controllerRevision
}

func (s statefulSetWorkload) resourceType() string             { return "statefulset" }
func (s statefulSetWorkload) template() corev1.PodTemplateSpec { return s.Spec.Template }
func (s statefulSetWorkload) selector() *metav1.LabelSelector  { return s.Spec.Selector }
func (s statefulSetWorkload) revision() int64                  { return s.current.revision }

// StatefulSet Pods are labelled with the ControllerRevision name.
func (s statefulSetWorkload) revisionHash() string { return s.Status.UpdateRevision }
func (s statefulSetWorkload) copy() workload {
	return statefulSetWorkload{StatefulSet: s.DeepCopy(), current: s.current}
}

func (s statefulSetWorkload) settled() bool {
	return s.DeletionTimestamp == nil && s.Status.ObservedGeneration >= s.Generation && s.Status.UpdateRevision != ""
}

func (s statefulSetWorkload) strategy() appsv1.StatefulSetUpdateStrategyType {
	if s.Spec.UpdateStrategy.Type == "" {
		return appsv1.RollingUpdateStatefulSetStrategyType
	}
	return s.Spec.UpdateStrategy.Type
}

// partition is the lowest ordinal a RollingUpdate updates; lower ordinals keep
// the previous revision.
func (s statefulSetWorkload) partition() int32 {
	if s.strategy() == appsv1.RollingUpdateStatefulSetStrategyType && s.Spec.UpdateStrategy.RollingUpdate != nil && s.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		return *s.Spec.UpdateStrategy.RollingUpdate.Partition
	}
	return 0
}

func (s statefulSetWorkload) desiredPods() int32 {
	return max(replicas(s.Spec.Replicas)-s.partition(), 0)
}

// StatefulSets have no progress deadline, so a rollout never stalls. It
// succeeds once the Pods it updates, and all others, are available. Only a
// complete RollingUpdate promotes the update revision to the current revision.
func (s statefulSetWorkload) rolloutStatus() string {
	desired := replicas(s.Spec.Replicas)
	if !s.settled() || s.Status.Replicas != desired || s.Status.AvailableReplicas != desired || s.Status.UpdatedReplicas < s.desiredPods() {
		return rolloutStarted
	}
	if s.strategy() == appsv1.RollingUpdateStatefulSetStrategyType && s.partition() == 0 && s.Status.CurrentRevision != s.Status.UpdateRevision {
		return rolloutStarted
	}
	return rolloutSucceeded
}

func (s statefulSetWorkload) putDiff(diff pcommon.Map) {
	diff.PutStr("update_strategy", string(s.strategy()))
	diff.PutInt("partition", int64(s.partition()))
}
