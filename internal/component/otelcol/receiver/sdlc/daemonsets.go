package sdlc

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DaemonSetsArguments configures DaemonSet rollout tracking.
type DaemonSetsArguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *DaemonSetsArguments) SetToDefault() {
	*args = DaemonSetsArguments{Enabled: true}
}

// daemonSetWorkload is a DaemonSet with its newest ControllerRevision. Unlike
// StatefulSets, DaemonSet status doesn't name the update revision.
type daemonSetWorkload struct {
	*appsv1.DaemonSet
	current controllerRevision
}

func newestRevision(revisions map[string]controllerRevision) controllerRevision {
	var newest controllerRevision
	for _, revision := range revisions {
		if revision.revision > newest.revision {
			newest = revision
		}
	}
	return newest
}

func (d daemonSetWorkload) resourceType() string             { return "daemonset" }
func (d daemonSetWorkload) template() corev1.PodTemplateSpec { return d.Spec.Template }
func (d daemonSetWorkload) selector() *metav1.LabelSelector  { return d.Spec.Selector }
func (d daemonSetWorkload) revision() int64                  { return d.current.revision }
func (d daemonSetWorkload) revisionHash() string             { return d.current.hash }
func (d daemonSetWorkload) desiredPods() int32               { return d.Status.DesiredNumberScheduled }
func (d daemonSetWorkload) copy() workload {
	return daemonSetWorkload{DaemonSet: d.DeepCopy(), current: d.current}
}

// The controller writes the revision before it reports the observed generation.
func (d daemonSetWorkload) settled() bool {
	return d.DeletionTimestamp == nil && d.Status.ObservedGeneration >= d.Generation
}

func (d daemonSetWorkload) strategy() appsv1.DaemonSetUpdateStrategyType {
	if d.Spec.UpdateStrategy.Type == "" {
		return appsv1.RollingUpdateDaemonSetStrategyType
	}
	return d.Spec.UpdateStrategy.Type
}

// DaemonSets have no progress deadline, so a rollout never stalls. A DaemonSet
// scheduled on no nodes succeeds immediately.
func (d daemonSetWorkload) rolloutStatus() string {
	desired := d.Status.DesiredNumberScheduled
	if d.settled() && d.Status.UpdatedNumberScheduled == desired && d.Status.NumberAvailable == desired && d.Status.NumberUnavailable == 0 {
		return rolloutSucceeded
	}
	return rolloutStarted
}

func (d daemonSetWorkload) putDiff(diff pcommon.Map) {
	diff.PutStr("update_strategy", string(d.strategy()))
}
