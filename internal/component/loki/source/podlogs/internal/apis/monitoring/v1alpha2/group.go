package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion is the group version used to register CRDs for this
	// package.
	SchemeGroupVersion = schema.GroupVersion{Group: "monitoring.grafana.com", Version: "v1alpha2"}

	// SchemeBuilder is used to add Go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is required by client packages.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&PodLogs{},
		&PodLogsList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
