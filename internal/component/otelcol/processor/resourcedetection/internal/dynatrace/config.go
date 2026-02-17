package dynatrace

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "dynatrace"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		HostName:       rac.ResourceAttributeConfig{Enabled: true},
		EntityHost:     rac.ResourceAttributeConfig{Enabled: true},
		SmartScapeHost: rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for dynatrace resource attributes.
type ResourceAttributesConfig struct {
	HostName       rac.ResourceAttributeConfig `alloy:"host.name,block,optional"`
	EntityHost     rac.ResourceAttributeConfig `alloy:"dt.entity.host,block,optional"`
	SmartScapeHost rac.ResourceAttributeConfig `alloy:"dt.smartscape.host,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"host.name":          r.HostName.Convert(),
		"dt.entity.host":     r.EntityHost.Convert(),
		"dt.smartscape.host": r.SmartScapeHost.Convert(),
	}
}
