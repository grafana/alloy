package scaleway

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "scaleway"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudAccountID:        rac.ResourceAttributeConfig{Enabled: true},
		CloudAvailabilityZone: rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:         rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:         rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:           rac.ResourceAttributeConfig{Enabled: true},
		HostID:                rac.ResourceAttributeConfig{Enabled: true},
		HostImageID:           rac.ResourceAttributeConfig{Enabled: true},
		HostImageName:         rac.ResourceAttributeConfig{Enabled: true},
		HostName:              rac.ResourceAttributeConfig{Enabled: true},
		HostType:              rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for Scaleway cloud resource attributes.
type ResourceAttributesConfig struct {
	CloudAccountID        rac.ResourceAttributeConfig `alloy:"cloud.account.id,block,optional"`
	CloudAvailabilityZone rac.ResourceAttributeConfig `alloy:"cloud.availability_zone,block,optional"`
	CloudPlatform         rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider         rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion           rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	HostID                rac.ResourceAttributeConfig `alloy:"host.id,block,optional"`
	HostImageID           rac.ResourceAttributeConfig `alloy:"host.image.id,block,optional"`
	HostImageName         rac.ResourceAttributeConfig `alloy:"host.image.name,block,optional"`
	HostName              rac.ResourceAttributeConfig `alloy:"host.name,block,optional"`
	HostType              rac.ResourceAttributeConfig `alloy:"host.type,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"cloud.account.id":        r.CloudAccountID.Convert(),
		"cloud.availability_zone": r.CloudAvailabilityZone.Convert(),
		"cloud.platform":          r.CloudPlatform.Convert(),
		"cloud.provider":          r.CloudProvider.Convert(),
		"cloud.region":            r.CloudRegion.Convert(),
		"host.id":                 r.HostID.Convert(),
		"host.image.id":           r.HostImageID.Convert(),
		"host.image.name":         r.HostImageName.Convert(),
		"host.name":               r.HostName.Convert(),
		"host.type":               r.HostType.Convert(),
	}
}
