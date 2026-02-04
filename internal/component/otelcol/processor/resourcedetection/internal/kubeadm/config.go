package kubeadm

import (
	"github.com/grafana/alloy/internal/component/otelcol"
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "kubeadm"

type Config struct {
	KubernetesAPIConfig otelcol.KubernetesAPIConfig `alloy:",squash"`
	ResourceAttributes  ResourceAttributesConfig    `alloy:"resource_attributes,block,optional"`
}

var DefaultArguments = Config{
	KubernetesAPIConfig: otelcol.KubernetesAPIConfig{
		AuthType: otelcol.KubernetesAPIConfig_AuthType_None,
	},
	ResourceAttributes: ResourceAttributesConfig{
		K8sClusterName: rac.ResourceAttributeConfig{Enabled: true},
		K8sClusterUID:  rac.ResourceAttributeConfig{Enabled: true},
	},
}

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (c *Config) SetToDefault() {
	*c = DefaultArguments
}

func (args Config) Convert() map[string]any {
	return map[string]any{
		"auth_type":           args.KubernetesAPIConfig.AuthType,
		"context":             args.KubernetesAPIConfig.Context,
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config for k8snode resource attributes.
type ResourceAttributesConfig struct {
	K8sClusterName rac.ResourceAttributeConfig `alloy:"k8s.cluster.name,block,optional"`
	K8sClusterUID  rac.ResourceAttributeConfig `alloy:"k8s.cluster.uid,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"k8s.cluster.name": r.K8sClusterName.Convert(),
		"k8s.cluster.uid":  r.K8sClusterUID.Convert(),
	}
}
