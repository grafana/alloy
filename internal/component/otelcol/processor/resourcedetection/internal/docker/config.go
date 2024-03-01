package docker

import (
	rac "github.com/grafana/agent/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/river"
)

const Name = "docker"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		HostName: rac.ResourceAttributeConfig{Enabled: true},
		OsType:   rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for docker resource attributes.
type ResourceAttributesConfig struct {
	HostName rac.ResourceAttributeConfig `river:"host.name,block,optional"`
	OsType   rac.ResourceAttributeConfig `river:"os.type,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"host.name": r.HostName.Convert(),
		"os.type":   r.OsType.Convert(),
	}
}
