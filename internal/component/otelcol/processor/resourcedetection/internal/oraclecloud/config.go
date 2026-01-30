package oraclecloud

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "oraclecloud"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudPlatform:         rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:         rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:           rac.ResourceAttributeConfig{Enabled: true},
		CloudAvailabilityZone: rac.ResourceAttributeConfig{Enabled: true},
		HostID:                rac.ResourceAttributeConfig{Enabled: true},
		HostName:              rac.ResourceAttributeConfig{Enabled: true},
		HostType:              rac.ResourceAttributeConfig{Enabled: true},
		K8sClusterName:        rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for oracle cloud resource attributes.
type ResourceAttributesConfig struct {
	CloudPlatform         rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider         rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion           rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	CloudAvailabilityZone rac.ResourceAttributeConfig `alloy:"cloud.availability_zone,block,optional"`
	HostID                rac.ResourceAttributeConfig `alloy:"host.id,block,optional"`
	HostName              rac.ResourceAttributeConfig `alloy:"host.name,block,optional"`
	HostType              rac.ResourceAttributeConfig `alloy:"host.type,block,optional"`
	K8sClusterName        rac.ResourceAttributeConfig `alloy:"k8s.cluster.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"cloud.platform":          r.CloudPlatform.Convert(),
		"cloud.provider":          r.CloudProvider.Convert(),
		"cloud.region":            r.CloudRegion.Convert(),
		"cloud.availability_zone": r.CloudAvailabilityZone.Convert(),
		"host.id":                 r.HostID.Convert(),
		"host.name":               r.HostName.Convert(),
		"host.type":               r.HostType.Convert(),
		"k8s.cluster.name":        r.K8sClusterName.Convert(),
	}
}
