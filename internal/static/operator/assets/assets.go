// Package assets contains helper types used for loading in static assets when
// configuring the Grafana Agent.
package assets

import (
	"fmt"

	prom_v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
)

// Key is a path-like identifier representing a Secret or ConfigMap value. It is
// used for looking up values during config generation that cannot be loaded
// directly from a file (e.g., BasicAuth Username).
//
// The naming convention is either:
//
//	/secrets/<namespace>/<name>/<key>
//
// or:
//
//	/configMaps/<namespace>/<name>/<key>
//
// Resources associated with a key should be watched for changes and trigger a
// reconcile when modified.
type Key string

// SecretStore is an in-memory cache for secrets, intended to be used for
// static secrets in generated configuration files.
type SecretStore map[Key]string

// KeyForSecret returns the key for a given namespace and a secret key
// selector.
func KeyForSecret(namespace string, sel *v1.SecretKeySelector) Key {
	if sel == nil {
		return Key("")
	}
	return Key(fmt.Sprintf("/secrets/%s/%s/%s", namespace, sel.Name, sel.Key))
}

// KeyForConfigMap returns the key for a given namespace and a config map
// key selector.
func KeyForConfigMap(namespace string, sel *v1.ConfigMapKeySelector) Key {
	if sel == nil {
		return Key("")
	}
	return Key(fmt.Sprintf("/configMaps/%s/%s/%s", namespace, sel.Name, sel.Key))
}

// KeyForSelector retrieves the key for a SecretOrConfigMap.
func KeyForSelector(namespace string, sel *prom_v1.SecretOrConfigMap) Key {
	switch {
	case sel.ConfigMap != nil:
		return KeyForConfigMap(namespace, sel.ConfigMap)
	case sel.Secret != nil:
		return KeyForSecret(namespace, sel.Secret)
	default:
		return Key("")
	}
}
