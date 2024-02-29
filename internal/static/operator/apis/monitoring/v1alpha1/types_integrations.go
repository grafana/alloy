package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IntegrationsSubsystemSpec defines global settings to apply across the
// integrations subsystem.
type IntegrationsSubsystemSpec struct {
	// Label selector to find Integration resources to run. When nil, no
	// integration resources will be defined.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Label selector for namespaces to search when discovering integration
	// resources. If nil, integration resources are only discovered in the
	// namespace of the GrafanaAgent resource.
	//
	// Set to `{}` to search all namespaces.
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path="integrations"
// +kubebuilder:resource:singular="integration"
// +kubebuilder:resource:categories="agent-operator"

// Integration runs a single Grafana Agent integration. Integrations that
// generate telemetry must be configured to send that telemetry somewhere, such
// as autoscrape for exporter-based integrations.
//
// Integrations have access to the LogsInstances and MetricsInstances in the
// same GrafanaAgent resource set, referenced by the <namespace>/<name> of the
// Instance resource.
//
// For example, if there is a default/production MetricsInstance, you can
// configure a supported integration's autoscrape block with:
//
//	autoscrape:
//	  enable: true
//	  metrics_instance: default/production
//
// There is currently no way for telemetry created by an Operator-managed
// integration to be collected from outside of the integration itself.
type Integration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specifies the desired behavior of the Integration.
	Spec IntegrationSpec `json:"spec,omitempty"`
}

// IntegrationSpec specifies the desired behavior of a metrics
// integration.
type IntegrationSpec struct {
	// Name of the integration to run (e.g., "node_exporter", "mysqld_exporter").
	Name string `json:"name"`

	// Type informs Grafana Agent Operator about how to manage the integration being
	// configured.
	Type IntegrationType `json:"type"`

	// +kubebuilder:validation:Type=object

	// The configuration for the named integration. Note that Integrations are
	// deployed with the integrations-next feature flag, which has different
	// common settings:
	//
	//   https://grafana.com/docs/agent/latest/configuration/integrations/integrations-next/
	Config apiextv1.JSON `json:"config"`

	// An extra list of Volumes to be associated with the Grafana Agent pods
	// running this integration. Volume names are mutated to be unique across
	// all Integrations. Note that the specified volumes should be able to
	// tolerate existing on multiple pods at once when type is daemonset.
	//
	// Don't use volumes for loading Secrets or ConfigMaps from the same namespace
	// as the Integration; use the Secrets and ConfigMaps fields instead.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// An extra list of VolumeMounts to be associated with the Grafana Agent pods
	// running this integration. VolumeMount names are mutated to be unique
	// across all used IntegrationSpecs.
	//
	// Mount paths should include the namespace/name of the Integration CR to
	// avoid potentially colliding with other resources.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// An extra list of keys from Secrets in the same namespace as the
	// Integration which will be mounted into the Grafana Agent pod running this
	// Integration.
	//
	// Secrets will be mounted at
	// /etc/grafana-agent/integrations/secrets/<secret_namespace>/<secret_name>/<key>.
	Secrets []corev1.SecretKeySelector `json:"secrets,omitempty"`

	// An extra list of keys from ConfigMaps in the same namespace as the
	// Integration which will be mounted into the Grafana Agent pod running this
	// Integration.
	//
	// ConfigMaps are mounted at
	// /etc/grafana-agent/integrations/configMaps/<configmap_namespace>/<configmap_name>/<key>.
	ConfigMaps []corev1.ConfigMapKeySelector `json:"configMaps,omitempty"`
}

// IntegrationType determines specific behaviors of a configured integration.
type IntegrationType struct {
	// +kubebuilder:validation:Optional

	// When true, the configured integration should be run on every Node in the
	// cluster. This is required for Integrations that generate Node-specific
	// metrics like node_exporter, otherwise it must be false to avoid generating
	// duplicate metrics.
	AllNodes bool `json:"allNodes"`

	// +kubebuilder:validation:Optional

	// Whether this integration can only be defined once for a Grafana Agent
	// process, such as statsd_exporter. It is invalid for a GrafanaAgent to
	// discover multiple unique Integrations with the same Integration name
	// (i.e., a single GrafanaAgent cannot deploy two statsd_exporters).
	Unique bool `json:"unique"`
}

// +kubebuilder:object:root=true

// IntegrationList is a list of Integration.
type IntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of Integration.
	Items []*Integration `json:"items"`
}
