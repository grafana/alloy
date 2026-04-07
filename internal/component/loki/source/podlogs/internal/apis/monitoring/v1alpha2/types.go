package v1alpha2

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/alloy/internal/component/loki/process/stages"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:path="podlogs"
// +kubebuilder:resource:categories="grafana-alloy"
// +kubebuilder:resource:categories="alloy"

// PodLogs defines how to collect logs for a Pod.
type PodLogs struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PodLogsSpec `json:"spec,omitempty"`
}

// PodLogsSpec defines how to collect logs for a Pod.
type PodLogsSpec struct {
	// Selector to select Pod objects. Required.
	Selector metav1.LabelSelector `json:"selector"`
	// Selector to select which namespaces the Pod objects are discovered from.
	NamespaceSelector metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// RelabelConfigs to apply to logs before delivering.
	RelabelConfigs []*promv1.RelabelConfig `json:"relabelings,omitempty"`

	// PipelineStages defines optional log processing stages applied to each
	// log line collected by this PodLogs before forwarding. Stages are applied
	// in order. Note: multiline, windowsevent, and eventlogmessage stages are
	// not supported because log lines from different pods share the same
	// pipeline and would be incorrectly merged.
	PipelineStages []stages.PodLogsStageConfig `json:"pipelineStages,omitempty"`
}

// +kubebuilder:object:root=true

// PodLogsList is a list of PodLogs.
type PodLogsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of PodLogs.
	Items []*PodLogs `json:"items"`
}
