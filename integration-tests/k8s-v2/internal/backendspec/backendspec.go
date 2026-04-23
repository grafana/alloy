// Package backendspec is the single source of truth for dependency/backend
// metadata (kubernetes namespace, service name, readiness path, ...). Both
// the deps installer and the assert port-forward helpers consume the same
// Spec so loki/mimir (and future backends) only need to be described once.
package backendspec

// Spec describes a dependency backend that the k8s-v2 harness installs and
// the per-test assertions port-forward to.
type Spec struct {
	// Name is a short identifier used in logs and error messages.
	Name string
	// Namespace is the kubernetes namespace the dependency lives in.
	Namespace string
	// Service is the kubernetes Service name used for port-forward.
	Service string
	// Deployment is the deployment name waited on for readiness.
	Deployment string
	// Port is the Service's target port.
	Port int
	// ReadinessPath is the HTTP path polled after port-forward to confirm
	// the backend is serving.
	ReadinessPath string
	// ManifestFile is the file name (under internal/deps/manifests) that
	// holds the kubernetes manifests for this backend.
	ManifestFile string
}

// Loki describes the Loki test backend.
var Loki = Spec{
	Name:          "loki",
	Namespace:     "loki",
	Service:       "loki",
	Deployment:    "loki",
	Port:          3100,
	ReadinessPath: "/ready",
	ManifestFile:  "loki.yaml",
}

// Mimir describes the Mimir test backend.
var Mimir = Spec{
	Name:          "mimir",
	Namespace:     "mimir",
	Service:       "mimir",
	Deployment:    "mimir",
	Port:          9009,
	ReadinessPath: "/ready",
	ManifestFile:  "mimir.yaml",
}
