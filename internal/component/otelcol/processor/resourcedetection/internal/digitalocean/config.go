package digitalocean

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "digitalocean"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudProvider: rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:   rac.ResourceAttributeConfig{Enabled: true},
		HostID:        rac.ResourceAttributeConfig{Enabled: true},
		HostName:      rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for DigitalOcean cloud resource attributes.
type ResourceAttributesConfig struct {
	CloudProvider rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion   rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	HostID        rac.ResourceAttributeConfig `alloy:"host.id,block,optional"`
	HostName      rac.ResourceAttributeConfig `alloy:"host.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"cloud.provider": r.CloudProvider.Convert(),
		"cloud.region":   r.CloudRegion.Convert(),
		"host.id":        r.HostID.Convert(),
		"host.name":      r.HostName.Convert(),
	}
}
