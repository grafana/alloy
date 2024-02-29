// +kubebuilder:object:generate=true
// +groupName=monitoring.grafana.com

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// SchemeGroupVersion is the group version used to register CRDs for this
	// package.
	SchemeGroupVersion = schema.GroupVersion{Group: "monitoring.grafana.com", Version: "v1alpha1"}

	// SchemeBuilder is used to add Go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme is required by client packages.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&GrafanaAgent{},
		&GrafanaAgentList{},
		&MetricsInstance{},
		&MetricsInstanceList{},
		&LogsInstance{},
		&LogsInstanceList{},
		&PodLogs{},
		&PodLogsList{},
		&Integration{},
		&IntegrationList{},
	)
}
