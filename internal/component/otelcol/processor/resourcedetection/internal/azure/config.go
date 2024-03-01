package azure

import (
	rac "github.com/grafana/agent/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/river"
)

const Name = "azure"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		AzureResourcegroupName: rac.ResourceAttributeConfig{Enabled: true},
		AzureVMName:            rac.ResourceAttributeConfig{Enabled: true},
		AzureVMScalesetName:    rac.ResourceAttributeConfig{Enabled: true},
		AzureVMSize:            rac.ResourceAttributeConfig{Enabled: true},
		CloudAccountID:         rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:          rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:          rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:            rac.ResourceAttributeConfig{Enabled: true},
		HostID:                 rac.ResourceAttributeConfig{Enabled: true},
		HostName:               rac.ResourceAttributeConfig{Enabled: true},
	},
}

var _ river.Defaulter = (*Config)(nil)

// SetToDefault implements river.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]interface{} {
	return map[string]interface{}{
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config for azure resource attributes.
type ResourceAttributesConfig struct {
	AzureResourcegroupName rac.ResourceAttributeConfig `river:"azure.resourcegroup.name,block,optional"`
	AzureVMName            rac.ResourceAttributeConfig `river:"azure.vm.name,block,optional"`
	AzureVMScalesetName    rac.ResourceAttributeConfig `river:"azure.vm.scaleset.name,block,optional"`
	AzureVMSize            rac.ResourceAttributeConfig `river:"azure.vm.size,block,optional"`
	CloudAccountID         rac.ResourceAttributeConfig `river:"cloud.account.id,block,optional"`
	CloudPlatform          rac.ResourceAttributeConfig `river:"cloud.platform,block,optional"`
	CloudProvider          rac.ResourceAttributeConfig `river:"cloud.provider,block,optional"`
	CloudRegion            rac.ResourceAttributeConfig `river:"cloud.region,block,optional"`
	HostID                 rac.ResourceAttributeConfig `river:"host.id,block,optional"`
	HostName               rac.ResourceAttributeConfig `river:"host.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"azure.resourcegroup.name": r.AzureResourcegroupName.Convert(),
		"azure.vm.name":            r.AzureVMName.Convert(),
		"azure.vm.scaleset.name":   r.AzureVMScalesetName.Convert(),
		"azure.vm.size":            r.AzureVMSize.Convert(),
		"cloud.account.id":         r.CloudAccountID.Convert(),
		"cloud.platform":           r.CloudPlatform.Convert(),
		"cloud.provider":           r.CloudProvider.Convert(),
		"cloud.region":             r.CloudRegion.Convert(),
		"host.id":                  r.HostID.Convert(),
		"host.name":                r.HostName.Convert(),
	}
}
