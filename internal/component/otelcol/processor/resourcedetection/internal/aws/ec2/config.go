package ec2

import (
	"time"

	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "ec2"

// Config defines user-specified configurations unique to the EC2 detector
type Config struct {
	// Tags is a list of regex's to match ec2 instance tag keys that users want
	// to add as resource attributes to processed data
	Tags                  []string                 `alloy:"tags,attr,optional"`
	ResourceAttributes    ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
	MaxAttempts           int                      `alloy:"max_attempts,attr,optional"`
	MaxBackoff            time.Duration            `alloy:"max_backoff,attr,optional"`
	FailOnMissingMetadata bool                     `alloy:"fail_on_missing_metadata,attr,optional"`
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
	MaxAttempts: 3,
	MaxBackoff:  20 * time.Second,
}

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]any {
	return map[string]any{
		"tags":                     append([]string{}, args.Tags...),
		"resource_attributes":      args.ResourceAttributes.Convert(),
		"max_attempts":             args.MaxAttempts,
		"max_backoff":              args.MaxBackoff,
		"fail_on_missing_metadata": args.FailOnMissingMetadata,
	}
}

// ResourceAttributesConfig provides config to enable and disable resource attributes.
type ResourceAttributesConfig struct {
	CloudAccountID        rac.ResourceAttributeConfig `alloy:"cloud.account.id,block,optional"`
	CloudAvailabilityZone rac.ResourceAttributeConfig `alloy:"cloud.availability_zone,block,optional"`
	CloudPlatform         rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider         rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion           rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	HostID                rac.ResourceAttributeConfig `alloy:"host.id,block,optional"`
	HostImageID           rac.ResourceAttributeConfig `alloy:"host.image.id,block,optional"`
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
		"host.name":               r.HostName.Convert(),
		"host.type":               r.HostType.Convert(),
	}
}
