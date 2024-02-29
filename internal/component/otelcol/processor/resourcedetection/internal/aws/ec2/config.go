package ec2

import (
	rac "github.com/grafana/agent/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/river"
)

const Name = "ec2"

// Config defines user-specified configurations unique to the EC2 detector
type Config struct {
	// Tags is a list of regex's to match ec2 instance tag keys that users want
	// to add as resource attributes to processed data
	Tags               []string                 `river:"tags,attr,optional"`
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
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
		HostName:              rac.ResourceAttributeConfig{Enabled: true},
		HostType:              rac.ResourceAttributeConfig{Enabled: true},
	},
}

var _ river.Defaulter = (*Config)(nil)

// SetToDefault implements river.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]interface{} {
	return map[string]interface{}{
		"tags":                append([]string{}, args.Tags...),
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config to enable and disable resource attributes.
type ResourceAttributesConfig struct {
	CloudAccountID        rac.ResourceAttributeConfig `river:"cloud.account.id,block,optional"`
	CloudAvailabilityZone rac.ResourceAttributeConfig `river:"cloud.availability_zone,block,optional"`
	CloudPlatform         rac.ResourceAttributeConfig `river:"cloud.platform,block,optional"`
	CloudProvider         rac.ResourceAttributeConfig `river:"cloud.provider,block,optional"`
	CloudRegion           rac.ResourceAttributeConfig `river:"cloud.region,block,optional"`
	HostID                rac.ResourceAttributeConfig `river:"host.id,block,optional"`
	HostImageID           rac.ResourceAttributeConfig `river:"host.image.id,block,optional"`
	HostName              rac.ResourceAttributeConfig `river:"host.name,block,optional"`
	HostType              rac.ResourceAttributeConfig `river:"host.type,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"cloud.account.id":        r.CloudAccountID.Convert(),
		"cloud.availability_zone": r.CloudAvailabilityZone.Convert(),
		"cloud.platform":          r.CloudPlatform.Convert(),
		"cloud.provider":          r.CloudProvider.Convert(),
		"cloud.region":            r.CloudRegion.Convert(),
		"host.id":                 r.HostID.Convert(),
		"host.image.id":           r.HostImageID.Convert(),
		"host.name":               r.HostName.Convert(),
		"host.type":               r.HostType.Convert(),
	}
}
