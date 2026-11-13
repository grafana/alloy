package sdlc

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// metadataRule leaves annotations that tools or Kubernetes set, such as
// last-applied-configuration, out of snapshots. Every other field keeps its
// value, including environment variable values and container arguments.
var metadataRule = &snapshotRule{fields: map[string]*snapshotRule{"annotations": annotationsRule}}

// podTemplateRule applies metadataRule to the pod template's metadata.
var podTemplateRule = &snapshotRule{fields: map[string]*snapshotRule{"metadata": metadataRule}}

// workloadSnapshot has the shape of a workload of kind: its metadata, and the
// pod template as spec.template.
func workloadSnapshot(kind resourceKind, metadata *metav1.ObjectMeta, template *corev1.PodTemplateSpec) map[string]any {
	out := typed(nil, kind)
	if m := snapshot(metadataRule, metadata); m != nil {
		out["metadata"] = m
	}
	if t := snapshot(podTemplateRule, template); t != nil {
		out["spec"] = map[string]any{"template": t}
	}
	return out
}
