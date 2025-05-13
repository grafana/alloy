package system

import (
	"fmt"

	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "system"

// Config defines user-specified configurations unique to the system detector
type Config struct {
	// The HostnameSources is a priority list of sources from which hostname will be fetched.
	// In case of the error in fetching hostname from source,
	// the next source from the list will be considered.
	HostnameSources []string `alloy:"hostname_sources,attr,optional"`

	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (c *Config) SetToDefault() {
	*c = Config{
		HostnameSources: []string{"dns", "os"},
		ResourceAttributes: ResourceAttributesConfig{
			HostArch:           rac.ResourceAttributeConfig{Enabled: false},
			HostCPUCacheL2Size: rac.ResourceAttributeConfig{Enabled: false},
			HostCPUFamily:      rac.ResourceAttributeConfig{Enabled: false},
			HostCPUModelID:     rac.ResourceAttributeConfig{Enabled: false},
			HostCPUModelName:   rac.ResourceAttributeConfig{Enabled: false},
			HostCPUStepping:    rac.ResourceAttributeConfig{Enabled: false},
			HostCPUVendorID:    rac.ResourceAttributeConfig{Enabled: false},
			HostID:             rac.ResourceAttributeConfig{Enabled: false},
			HostIP:             rac.ResourceAttributeConfig{Enabled: false},
			HostMac:            rac.ResourceAttributeConfig{Enabled: false},
			HostName:           rac.ResourceAttributeConfig{Enabled: true},
			OsDescription:      rac.ResourceAttributeConfig{Enabled: false},
			OsType:             rac.ResourceAttributeConfig{Enabled: true},
		},
	}
}

// Validate config
func (cfg *Config) Validate() error {
	for _, hostnameSource := range cfg.HostnameSources {
		switch hostnameSource {
		case "os", "dns", "cname", "lookup":
			// Valid option - nothing to do
		default:
			return fmt.Errorf("invalid hostname source: %s", hostnameSource)
		}
	}
	return nil
}

func (args Config) Convert() map[string]interface{} {
	return map[string]interface{}{
		"hostname_sources":    args.HostnameSources,
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config for system resource attributes.
type ResourceAttributesConfig struct {
	HostArch           rac.ResourceAttributeConfig `alloy:"host.arch,block,optional"`
	HostCPUCacheL2Size rac.ResourceAttributeConfig `alloy:"host.cpu.cache.l2.size,block,optional"`
	HostCPUFamily      rac.ResourceAttributeConfig `alloy:"host.cpu.family,block,optional"`
	HostCPUModelID     rac.ResourceAttributeConfig `alloy:"host.cpu.model.id,block,optional"`
	HostCPUModelName   rac.ResourceAttributeConfig `alloy:"host.cpu.model.name,block,optional"`
	HostCPUStepping    rac.ResourceAttributeConfig `alloy:"host.cpu.stepping,block,optional"`
	HostCPUVendorID    rac.ResourceAttributeConfig `alloy:"host.cpu.vendor.id,block,optional"`
	HostID             rac.ResourceAttributeConfig `alloy:"host.id,block,optional"`
	HostIP             rac.ResourceAttributeConfig `alloy:"host.ip,block,optional"`
	HostMac            rac.ResourceAttributeConfig `alloy:"host.mac,block,optional"`
	HostName           rac.ResourceAttributeConfig `alloy:"host.name,block,optional"`
	OsDescription      rac.ResourceAttributeConfig `alloy:"os.description,block,optional"`
	OsType             rac.ResourceAttributeConfig `alloy:"os.type,block,optional"`
	OsVersion          rac.ResourceAttributeConfig `alloy:"os.version,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"host.arch":              r.HostArch.Convert(),
		"host.cpu.cache.l2.size": r.HostCPUCacheL2Size.Convert(),
		"host.cpu.family":        r.HostCPUFamily.Convert(),
		"host.cpu.model.id":      r.HostCPUModelID.Convert(),
		"host.cpu.model.name":    r.HostCPUModelName.Convert(),
		"host.cpu.stepping":      r.HostCPUStepping.Convert(),
		"host.cpu.vendor.id":     r.HostCPUVendorID.Convert(),
		"host.id":                r.HostID.Convert(),
		"host.ip":                r.HostIP.Convert(),
		"host.mac":               r.HostMac.Convert(),
		"host.name":              r.HostName.Convert(),
		"os.description":         r.OsDescription.Convert(),
		"os.type":                r.OsType.Convert(),
		"os.version":             r.OsVersion.Convert(),
	}
}
