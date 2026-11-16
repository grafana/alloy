package sdlc

// resourceKind is a kind of object the receiver reports: its
// provider.resource.type, and the apiVersion and kind of its snapshots, which
// objects from informers don't carry.
type resourceKind struct {
	resourceType, apiVersion, kind string
}

var (
	deploymentKind  = resourceKind{resourceType: "deployment", apiVersion: "apps/v1", kind: "Deployment"}
	statefulSetKind = resourceKind{resourceType: "statefulset", apiVersion: "apps/v1", kind: "StatefulSet"}
	daemonSetKind   = resourceKind{resourceType: "daemonset", apiVersion: "apps/v1", kind: "DaemonSet"}
	configMapKind   = resourceKind{resourceType: "configmap", apiVersion: "v1", kind: "ConfigMap"}
	secretKind      = resourceKind{resourceType: "secret", apiVersion: "v1", kind: "Secret"}
)

var resourceKinds = []resourceKind{deploymentKind, statefulSetKind, daemonSetKind, configMapKind, secretKind}
