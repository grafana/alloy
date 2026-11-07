package v1alpha2

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Pipeline runs on every collected log line before it is forwarded.
	Pipeline *PipelineSpec `json:"pipeline,omitempty"`
}

// PipelineSpec defines the stages a log line passes through, in order.
type PipelineSpec struct {
	// Stages to run on each log line, in order.
	// +kubebuilder:validation:MinItems=1
	Stages []PipelineStage `json:"stages"`
}

// PipelineStage is a single processing stage. Exactly one field must be set.
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:MaxProperties=1
type PipelineStage struct {
	// Drop discards log lines matching the configured source, value or expression.
	Drop *DropStage `json:"drop,omitempty"`
	// Regex extracts data from the log line into the extracted map using named capture groups.
	Regex *RegexStage `json:"regex,omitempty"`
	// Labels promotes keys from the extracted map to labels.
	Labels *LabelsStage `json:"labels,omitempty"`
	// StaticLabels adds fixed labels to every log line.
	StaticLabels *StaticLabelsStage `json:"staticLabels,omitempty"`
}

// DropStage mirrors the drop stage of loki.process.
type DropStage struct {
	// Source is a comma-separated list of extracted keys to match on. When empty, the log line is matched.
	Source string `json:"source,omitempty"`
	// Value drops the line when the joined source values equal it exactly.
	Value string `json:"value,omitempty"`
	// Separator joins multiple source values. Defaults to ";".
	Separator string `json:"separator,omitempty"`
	// Expression drops the line when the RE2 regular expression matches.
	Expression string `json:"expression,omitempty"`
	// DropCounterReason is the reason label on the dropped lines counter. Defaults to "drop_stage".
	DropCounterReason string `json:"dropCounterReason,omitempty"`
}

// RegexStage mirrors the regex stage of loki.process.
type RegexStage struct {
	// Expression is an RE2 regular expression with named capture groups.
	Expression string `json:"expression"`
	// Source is the extracted key to run the expression on. When unset, the log line is used.
	Source string `json:"source,omitempty"`
	// LabelsFromGroups adds every named capture group as a label.
	LabelsFromGroups bool `json:"labelsFromGroups,omitempty"`
}

// LabelsStage mirrors the labels stage of loki.process.
type LabelsStage struct {
	// Values maps label names to extracted keys. An empty value uses the label name as the key.
	Values map[string]string `json:"values"`
}

// StaticLabelsStage mirrors the static_labels stage of loki.process.
type StaticLabelsStage struct {
	// Values maps label names to label values.
	Values map[string]string `json:"values"`
}

// +kubebuilder:object:root=true

// PodLogsList is a list of PodLogs.
type PodLogsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of PodLogs.
	Items []*PodLogs `json:"items"`
}
