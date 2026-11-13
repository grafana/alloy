package sdlc

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// podTemplateRule redacts pod templates in snapshots. Fields keep their values
// unless they're known to hold credentials: environment variable values,
// commands and arguments, HTTP header values and query strings of probes and
// lifecycle hooks, and volume driver options.
var podTemplateRule = func() *snapshotRule {
	handler := &snapshotRule{fields: map[string]*snapshotRule{
		"exec": {fields: map[string]*snapshotRule{"command": redactRule}},
		"httpGet": {fields: map[string]*snapshotRule{
			"path":        redactQueryRule,
			"httpHeaders": {items: &snapshotRule{fields: map[string]*snapshotRule{"value": redactRule}}},
		}},
	}}
	container := &snapshotRule{fields: map[string]*snapshotRule{
		"command":        redactRule,
		"args":           redactRule,
		"env":            {items: &snapshotRule{fields: map[string]*snapshotRule{"value": redactRule}}},
		"livenessProbe":  handler,
		"readinessProbe": handler,
		"startupProbe":   handler,
		"lifecycle": {fields: map[string]*snapshotRule{
			"postStart": handler,
			"preStop":   handler,
		}},
	}}
	return &snapshotRule{fields: map[string]*snapshotRule{
		"metadata": {fields: map[string]*snapshotRule{
			"annotations": annotationsRule,
		}},
		"spec": {fields: map[string]*snapshotRule{
			"containers":     {items: container},
			"initContainers": {items: container},
			// Some CSI and FlexVolume drivers take credentials as options.
			"volumes": {items: &snapshotRule{fields: map[string]*snapshotRule{
				"csi":        {fields: map[string]*snapshotRule{"volumeAttributes": redactRule}},
				"flexVolume": {fields: map[string]*snapshotRule{"options": redactRule}},
			}}},
		}},
	}}
}()

// workloadMetadataRule redacts workload metadata in snapshots.
var workloadMetadataRule = &snapshotRule{fields: map[string]*snapshotRule{"annotations": annotationsRule}}

// workloadSnapshot has the shape of a workload of kind: its metadata, and the
// redacted pod template as spec.template.
func workloadSnapshot(kind resourceKind, metadata *metav1.ObjectMeta, template *corev1.PodTemplateSpec) map[string]any {
	out := typed(nil, kind)
	if m := snapshot(workloadMetadataRule, metadata); m != nil {
		out["metadata"] = m
	}
	if t := snapshot(podTemplateRule, template); t != nil {
		out["spec"] = map[string]any{"template": t}
	}
	return out
}
