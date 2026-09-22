package appservice

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "azureappservice"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		AzureAppServiceInstanceID: rac.ResourceAttributeConfig{Enabled: true},
		AzureResourceGroupName:    rac.ResourceAttributeConfig{Enabled: true},
		CloudAccountID:            rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:             rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:             rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:               rac.ResourceAttributeConfig{Enabled: true},
		CloudResourceID:           rac.ResourceAttributeConfig{Enabled: true},
		DeploymentEnvironmentName: rac.ResourceAttributeConfig{Enabled: true},
		ServiceName:               rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for azureappservice resource attributes.
type ResourceAttributesConfig struct {
	AzureAppServiceInstanceID rac.ResourceAttributeConfig `alloy:"azure.app_service.instance.id,block,optional"`
	AzureResourceGroupName    rac.ResourceAttributeConfig `alloy:"azure.resource_group.name,block,optional"`
	CloudAccountID            rac.ResourceAttributeConfig `alloy:"cloud.account.id,block,optional"`
	CloudPlatform             rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider             rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion               rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	CloudResourceID           rac.ResourceAttributeConfig `alloy:"cloud.resource_id,block,optional"`
	DeploymentEnvironmentName rac.ResourceAttributeConfig `alloy:"deployment.environment.name,block,optional"`
	ServiceName               rac.ResourceAttributeConfig `alloy:"service.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"azure.app_service.instance.id": r.AzureAppServiceInstanceID.Convert(),
		"azure.resource_group.name":     r.AzureResourceGroupName.Convert(),
		"cloud.account.id":              r.CloudAccountID.Convert(),
		"cloud.platform":                r.CloudPlatform.Convert(),
		"cloud.provider":                r.CloudProvider.Convert(),
		"cloud.region":                  r.CloudRegion.Convert(),
		"cloud.resource_id":             r.CloudResourceID.Convert(),
		"deployment.environment.name":   r.DeploymentEnvironmentName.Convert(),
		"service.name":                  r.ServiceName.Convert(),
	}
}
