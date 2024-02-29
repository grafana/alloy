package config

import (
	"github.com/grafana/agent/internal/util/structwalk"
	prom "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AssetReference is a namespaced Secret or ConfigMap selector.
type AssetReference struct {
	Namespace string
	Reference prom.SecretOrConfigMap
}

// AssetReferences returns all secret or configmap selectors used throughout v.
func AssetReferences(v interface{}) []AssetReference {
	var refs []AssetReference
	w := assetReferencesWalker{
		addReference: func(ar AssetReference) {
			refs = append(refs, ar)
		},
	}
	structwalk.Walk(&w, v)
	return refs
}

type assetReferencesWalker struct {
	namespace    string
	addReference func(ar AssetReference)
}

func (arw *assetReferencesWalker) Visit(v interface{}) (w structwalk.Visitor) {
	if v == nil {
		return nil
	}

	// If we've come across a namespaced object, create a new visitor for that
	// namespace.
	if o, ok := v.(metav1.Object); ok {
		return &assetReferencesWalker{
			namespace:    o.GetNamespace(),
			addReference: arw.addReference,
		}
	}

	switch sel := v.(type) {
	case corev1.SecretKeySelector:
		if sel.Key != "" && sel.Name != "" {
			arw.addReference(AssetReference{
				Namespace: arw.namespace,
				Reference: prom.SecretOrConfigMap{Secret: &sel},
			})
		}
	case *corev1.SecretKeySelector:
		arw.addReference(AssetReference{
			Namespace: arw.namespace,
			Reference: prom.SecretOrConfigMap{Secret: sel},
		})
	case corev1.ConfigMapKeySelector:
		if sel.Key != "" && sel.Name != "" {
			arw.addReference(AssetReference{
				Namespace: arw.namespace,
				Reference: prom.SecretOrConfigMap{ConfigMap: &sel},
			})
		}
	case *corev1.ConfigMapKeySelector:
		arw.addReference(AssetReference{
			Namespace: arw.namespace,
			Reference: prom.SecretOrConfigMap{ConfigMap: sel},
		})
	}

	return arw
}
