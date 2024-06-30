package azure

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "azure"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
	Tags               []string                 `alloy:"tags,attr,optional"`
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

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]interface{} {
	return map[string]interface{}{
		"resource_attributes": args.ResourceAttributes.Convert(),
		"tags":                args.Tags,
	}
}

// ResourceAttributesConfig provides config for azure resource attributes.
type ResourceAttributesConfig struct {
	AzureResourcegroupName rac.ResourceAttributeConfig `alloy:"azure.resourcegroup.name,block,optional"`
	AzureVMName            rac.ResourceAttributeConfig `alloy:"azure.vm.name,block,optional"`
	AzureVMScalesetName    rac.ResourceAttributeConfig `alloy:"azure.vm.scaleset.name,block,optional"`
	AzureVMSize            rac.ResourceAttributeConfig `alloy:"azure.vm.size,block,optional"`
	CloudAccountID         rac.ResourceAttributeConfig `alloy:"cloud.account.id,block,optional"`
	CloudPlatform          rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider          rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion            rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	HostID                 rac.ResourceAttributeConfig `alloy:"host.id,block,optional"`
	HostName               rac.ResourceAttributeConfig `alloy:"host.name,block,optional"`
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
