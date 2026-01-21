package elasticbeanstalk

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "elasticbeanstalk"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudPlatform:         rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:         rac.ResourceAttributeConfig{Enabled: true},
		DeploymentEnvironment: rac.ResourceAttributeConfig{Enabled: true},
		ServiceInstanceID:     rac.ResourceAttributeConfig{Enabled: true},
		ServiceVersion:        rac.ResourceAttributeConfig{Enabled: true},
	},
}

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]any {
	return map[string]any{
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config for elastic_beanstalk resource attributes.
type ResourceAttributesConfig struct {
	CloudPlatform         rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider         rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	DeploymentEnvironment rac.ResourceAttributeConfig `alloy:"deployment.environment,block,optional"`
	ServiceInstanceID     rac.ResourceAttributeConfig `alloy:"service.instance.id,block,optional"`
	ServiceVersion        rac.ResourceAttributeConfig `alloy:"service.version,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"cloud.platform":         r.CloudPlatform.Convert(),
		"cloud.provider":         r.CloudProvider.Convert(),
		"deployment.environment": r.DeploymentEnvironment.Convert(),
		"service.instance.id":    r.ServiceInstanceID.Convert(),
		"service.version":        r.ServiceVersion.Convert(),
	}
}
