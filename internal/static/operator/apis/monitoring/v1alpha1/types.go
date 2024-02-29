package v1alpha1

import (
	prom_v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:path="grafanaagents"
// +kubebuilder:resource:singular="grafanaagent"
// +kubebuilder:resource:categories="agent-operator"

// GrafanaAgent defines a Grafana Agent deployment.
type GrafanaAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the specification of the desired behavior for the Grafana Agent
	// cluster.
	Spec GrafanaAgentSpec `json:"spec,omitempty"`
}

// MetricsInstanceSelector returns a selector to find MetricsInstances.
func (a *GrafanaAgent) MetricsInstanceSelector() ObjectSelector {
	return ObjectSelector{
		ObjectType:        &MetricsInstance{},
		ParentNamespace:   a.Namespace,
		NamespaceSelector: a.Spec.Metrics.InstanceNamespaceSelector,
		Labels:            a.Spec.Metrics.InstanceSelector,
	}
}

// LogsInstanceSelector returns a selector to find LogsInstances.
func (a *GrafanaAgent) LogsInstanceSelector() ObjectSelector {
	return ObjectSelector{
		ObjectType:        &LogsInstance{},
		ParentNamespace:   a.Namespace,
		NamespaceSelector: a.Spec.Logs.InstanceNamespaceSelector,
		Labels:            a.Spec.Logs.InstanceSelector,
	}
}

// IntegrationsSelector returns a selector to find Integrations.
func (a *GrafanaAgent) IntegrationsSelector() ObjectSelector {
	return ObjectSelector{
		ObjectType:        &Integration{},
		ParentNamespace:   a.Namespace,
		NamespaceSelector: a.Spec.Integrations.NamespaceSelector,
		Labels:            a.Spec.Integrations.Selector,
	}
}

// +kubebuilder:object:root=true

// GrafanaAgentList is a list of GrafanaAgents.
type GrafanaAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of GrafanaAgents.
	Items []*GrafanaAgent `json:"items"`
}

// GrafanaAgentSpec is a specification of the desired behavior of the Grafana
// Agent cluster.
type GrafanaAgentSpec struct {
	// LogLevel controls the log level of the generated pods. Defaults to "info" if not set.
	LogLevel string `json:"logLevel,omitempty"`
	// LogFormat controls the logging format of the generated pods. Defaults to "logfmt" if not set.
	LogFormat string `json:"logFormat,omitempty"`
	// APIServerConfig lets you specify a host and auth methods to access the
	// Kubernetes API server. If left empty, the Agent assumes that it is
	// running inside of the cluster and will discover API servers automatically
	// and use the pod's CA certificate and bearer token file at
	// /var/run/secrets/kubernetes.io/serviceaccount.
	APIServerConfig *prom_v1.APIServerConfig `json:"apiServer,omitempty"`
	// PodMetadata configures Labels and Annotations which are propagated to
	// created Grafana Agent pods.
	PodMetadata *prom_v1.EmbeddedObjectMetadata `json:"podMetadata,omitempty"`
	// Version of Grafana Agent to be deployed.
	Version string `json:"version,omitempty"`
	// Paused prevents actions except for deletion to be performed on the
	// underlying managed objects.
	Paused bool `json:"paused,omitempty"`
	// Image, when specified, overrides the image used to run Agent. Specify
	// the image along with a tag. You still need to set the version to ensure
	// Grafana Agent Operator knows which version of Grafana Agent is being
	// configured.
	Image *string `json:"image,omitempty"`

	// Version of Config Reloader to be deployed.
	ConfigReloaderVersion string `json:"configReloaderVersion,omitempty"`
	// Image, when specified, overrides the image used to run Config Reloader. Specify
	// the image along with a tag. You still need to set the version to ensure
	// Grafana Agent Operator knows which version of Grafana Agent is being
	// configured.
	ConfigReloaderImage *string `json:"configReloaderImage,omitempty"`

	// ImagePullSecrets holds an optional list of references to Secrets within
	// the same namespace used for pulling the Grafana Agent image from
	// registries.
	// More info: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// Storage spec to specify how storage will be used.
	Storage *prom_v1.StorageSpec `json:"storage,omitempty"`
	// Volumes allows configuration of additional volumes on the output
	// StatefulSet definition. The volumes specified are appended to other
	// volumes that are generated as a result of StorageSpec objects.
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// VolumeMounts lets you configure additional VolumeMounts on the output
	// StatefulSet definition. Specified VolumeMounts are appended to other
	// VolumeMounts generated as a result of StorageSpec objects
	// in the Grafana Agent container.
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// Resources holds requests and limits for individual pods.
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector defines which nodes pods should be scheduling on.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to use for running Grafana Agent pods.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Secrets is a list of secrets in the same namespace as the GrafanaAgent
	// object which will be mounted into each running Grafana Agent pod.
	// The secrets are mounted into /var/lib/grafana-agent/extra-secrets/<secret-name>.
	Secrets []string `json:"secrets,omitempty"`
	// ConfigMaps is a list of config maps in the same namespace as the
	// GrafanaAgent object which will be mounted into each running Grafana Agent
	// pod.
	// The ConfigMaps are mounted into /var/lib/grafana-agent/extra-configmaps/<configmap-name>.
	ConfigMaps []string `json:"configMaps,omitempty"`
	// Affinity, if specified, controls pod scheduling constraints.
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// Tolerations, if specified, controls the pod's tolerations.
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// TopologySpreadConstraints, if specified, controls the pod's topology spread constraints.
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// SecurityContext holds pod-level security attributes and common container
	// settings. When unspecified, defaults to the default PodSecurityContext.
	SecurityContext *v1.PodSecurityContext `json:"securityContext,omitempty"`
	// Containers lets you inject additional containers or modify operator-generated
	// containers. This can be used to add an authentication
	// proxy to a Grafana Agent pod or to change the behavior of an
	// operator-generated container. Containers described here modify an
	// operator-generated container if they share the same name and if modifications are done
	// via a strategic merge patch. The current container names are:
	// `grafana-agent` and `config-reloader`. Overriding containers is entirely
	// outside the scope of what the Grafana Agent team supports and by doing
	// so, you accept that this behavior may break at any time without notice.
	Containers []v1.Container `json:"containers,omitempty"`
	// InitContainers let you add initContainers to the pod definition. These
	// can be used to, for example, fetch secrets for injection into the Grafana
	// Agent configuration from external sources. Errors during the execution
	// of an initContainer cause the pod to restart.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
	// Using initContainers for any use case other than secret fetching is
	// entirely outside the scope of what the Grafana Agent maintainers
	// support and by doing so, you accept that this behavior may break at any
	// time without notice.
	InitContainers []v1.Container `json:"initContainers,omitempty"`
	// PriorityClassName is the priority class assigned to pods.
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// RuntimeClassName is the runtime class assigned to pods.
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// Port name used for the pods and governing service. This defaults to agent-metrics.
	PortName string `json:"portName,omitempty"`

	// Metrics controls the metrics subsystem of the Agent and settings
	// unique to metrics-specific pods that are deployed.
	Metrics MetricsSubsystemSpec `json:"metrics,omitempty"`

	// Logs controls the logging subsystem of the Agent and settings unique to
	// logging-specific pods that are deployed.
	Logs LogsSubsystemSpec `json:"logs,omitempty"`

	// Integrations controls the integration subsystem of the Agent and settings
	// unique to deployed integration-specific pods.
	Integrations IntegrationsSubsystemSpec `json:"integrations,omitempty"`

	// enableConfigReadAPI enables the read API for viewing the currently running
	// config port 8080 on the agent.
	// +kubebuilder:default=false
	EnableConfigReadAPI bool `json:"enableConfigReadAPI,omitempty"`

	// disableReporting disables reporting of enabled feature flags to Grafana.
	// +kubebuilder:default=false
	DisableReporting bool `json:"disableReporting,omitempty"`

	// disableSupportBundle disables the generation of support bundles.
	// +kubebuilder:default=false
	DisableSupportBundle bool `json:"disableSupportBundle,omitempty"`
}

// +kubebuilder:object:generate=false

// ObjectSelector is a set of selectors to use for finding an object in the
// resource hierarchy. When NamespaceSelector is nil, search for objects
// directly in the ParentNamespace.
type ObjectSelector struct {
	ObjectType        client.Object
	ParentNamespace   string
	NamespaceSelector *metav1.LabelSelector
	Labels            *metav1.LabelSelector
}
