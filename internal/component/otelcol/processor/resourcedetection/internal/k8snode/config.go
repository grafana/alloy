package k8snode

import (
	"github.com/grafana/agent/internal/component/otelcol"
	rac "github.com/grafana/agent/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/river"
)

const Name = "kubernetes_node"

type Config struct {
	KubernetesAPIConfig otelcol.KubernetesAPIConfig `river:",squash"`
	// NodeFromEnv can be used to extract the node name from an environment
	// variable. The value must be the name of the environment variable.
	// This is useful when the node where an Agent will run on cannot be
	// predicted. In such cases, the Kubernetes downward API can be used to
	// add the node name to each pod as an environment variable. K8s tagger
	// can then read this value and filter pods by it.
	//
	// For example, node name can be passed to each agent with the downward API as follows
	//
	// env:
	//   - name: K8S_NODE_NAME
	//     valueFrom:
	//       fieldRef:
	//         fieldPath: spec.nodeName
	//
	// Then the NodeFromEnv field can be set to `K8S_NODE_NAME` to filter all pods by the node that
	// the agent is running on.
	//
	// More on downward API here: https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/
	NodeFromEnvVar     string                   `river:"node_from_env_var,attr,optional"`
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
}

var DefaultArguments = Config{
	KubernetesAPIConfig: otelcol.KubernetesAPIConfig{
		AuthType: otelcol.KubernetesAPIConfig_AuthType_None,
	},
	NodeFromEnvVar: "K8S_NODE_NAME",
	ResourceAttributes: ResourceAttributesConfig{
		K8sNodeName: rac.ResourceAttributeConfig{Enabled: true},
		K8sNodeUID:  rac.ResourceAttributeConfig{Enabled: true},
	},
}

var _ river.Defaulter = (*Config)(nil)

// SetToDefault implements river.Defaulter.
func (c *Config) SetToDefault() {
	*c = DefaultArguments
}

func (args Config) Convert() map[string]interface{} {
	return map[string]interface{}{
		//TODO: K8sAPIConfig is squashed - is there a better way to "convert" it?
		"auth_type":           args.KubernetesAPIConfig.AuthType,
		"context":             args.KubernetesAPIConfig.Context,
		"node_from_env_var":   args.NodeFromEnvVar,
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config for k8snode resource attributes.
type ResourceAttributesConfig struct {
	K8sNodeName rac.ResourceAttributeConfig `river:"k8s.node.name,block,optional"`
	K8sNodeUID  rac.ResourceAttributeConfig `river:"k8s.node.uid,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"k8s.node.name": r.K8sNodeName.Convert(),
		"k8s.node.uid":  r.K8sNodeUID.Convert(),
	}
}
