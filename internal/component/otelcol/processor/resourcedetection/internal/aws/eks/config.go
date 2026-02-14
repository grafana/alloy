package eks

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "eks"

// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/resourcedetectionprocessor/internal/aws/eks/config.schema.yaml
type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
	NodeFromEnvVar     string                   `alloy:"node_from_env_var,attr,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudAccountID: rac.ResourceAttributeConfig{Enabled: false},
		CloudPlatform:  rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:  rac.ResourceAttributeConfig{Enabled: true},
		K8sClusterName: rac.ResourceAttributeConfig{Enabled: false},
	},
	NodeFromEnvVar: "K8S_NODE_NAME",
}

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]any {
	return map[string]any{
		"resource_attributes": args.ResourceAttributes.Convert(),
		"node_from_env_var":   args.NodeFromEnvVar,
	}
}

// ResourceAttributesConfig provides config for eks resource attributes.
type ResourceAttributesConfig struct {
	CloudAccountID rac.ResourceAttributeConfig `alloy:"cloud.account.id,block,optional"`
	CloudPlatform  rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider  rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	K8sClusterName rac.ResourceAttributeConfig `alloy:"k8s.cluster.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"cloud.account.id": r.CloudAccountID.Convert(),
		"cloud.platform":   r.CloudPlatform.Convert(),
		"cloud.provider":   r.CloudProvider.Convert(),
		"k8s.cluster.name": r.K8sClusterName.Convert(),
	}
}
