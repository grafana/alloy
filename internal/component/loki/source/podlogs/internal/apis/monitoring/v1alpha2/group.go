package v1alpha2

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// SchemeGroupVersion is the group version used to register CRDs for this
	// package.
	SchemeGroupVersion = schema.GroupVersion{Group: "monitoring.grafana.com", Version: "v1alpha2"}

	// SchemeBuilder is used to add Go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme is required by client packages.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&PodLogs{},
		&PodLogsList{},
	)
}
