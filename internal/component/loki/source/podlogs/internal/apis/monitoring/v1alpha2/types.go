package v1alpha2

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:path="podlogs"
// +kubebuilder:resource:path="podlogs"
// +kubebuilder:resource:categories="grafana-agent"

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
}

// +kubebuilder:object:root=true

// PodLogsList is a list of PodLogs.
type PodLogsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of PodLogs.
	Items []*PodLogs `json:"items"`
}
