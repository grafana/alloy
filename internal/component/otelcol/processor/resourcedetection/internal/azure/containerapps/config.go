package containerapps

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "azurecontainerapps"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		AzureContainerAppInstanceID: rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:               rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:               rac.ResourceAttributeConfig{Enabled: true},
		ServiceName:                 rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for azurecontainerapps resource attributes.
type ResourceAttributesConfig struct {
	AzureContainerAppInstanceID rac.ResourceAttributeConfig `alloy:"azure.container_app.instance.id,block,optional"`
	CloudPlatform               rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider               rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	ServiceName                 rac.ResourceAttributeConfig `alloy:"service.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"azure.container_app.instance.id": r.AzureContainerAppInstanceID.Convert(),
		"cloud.platform":                  r.CloudPlatform.Convert(),
		"cloud.provider":                  r.CloudProvider.Convert(),
		"service.name":                    r.ServiceName.Convert(),
	}
}
