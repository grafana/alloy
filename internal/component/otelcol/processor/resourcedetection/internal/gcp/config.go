package gcp

import (
	rac "github.com/grafana/agent/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/river"
)

const Name = "gcp"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudAccountID:          rac.ResourceAttributeConfig{Enabled: true},
		CloudAvailabilityZone:   rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:           rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:           rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:             rac.ResourceAttributeConfig{Enabled: true},
		FaasID:                  rac.ResourceAttributeConfig{Enabled: true},
		FaasInstance:            rac.ResourceAttributeConfig{Enabled: true},
		FaasName:                rac.ResourceAttributeConfig{Enabled: true},
		FaasVersion:             rac.ResourceAttributeConfig{Enabled: true},
		GcpCloudRunJobExecution: rac.ResourceAttributeConfig{Enabled: true},
		GcpCloudRunJobTaskIndex: rac.ResourceAttributeConfig{Enabled: true},
		GcpGceInstanceHostname:  rac.ResourceAttributeConfig{Enabled: false},
		GcpGceInstanceName:      rac.ResourceAttributeConfig{Enabled: false},
		HostID:                  rac.ResourceAttributeConfig{Enabled: true},
		HostName:                rac.ResourceAttributeConfig{Enabled: true},
		HostType:                rac.ResourceAttributeConfig{Enabled: true},
		K8sClusterName:          rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for gcp resource attributes.
type ResourceAttributesConfig struct {
	CloudAccountID          rac.ResourceAttributeConfig `river:"cloud.account.id,block,optional"`
	CloudAvailabilityZone   rac.ResourceAttributeConfig `river:"cloud.availability_zone,block,optional"`
	CloudPlatform           rac.ResourceAttributeConfig `river:"cloud.platform,block,optional"`
	CloudProvider           rac.ResourceAttributeConfig `river:"cloud.provider,block,optional"`
	CloudRegion             rac.ResourceAttributeConfig `river:"cloud.region,block,optional"`
	FaasID                  rac.ResourceAttributeConfig `river:"faas.id,block,optional"`
	FaasInstance            rac.ResourceAttributeConfig `river:"faas.instance,block,optional"`
	FaasName                rac.ResourceAttributeConfig `river:"faas.name,block,optional"`
	FaasVersion             rac.ResourceAttributeConfig `river:"faas.version,block,optional"`
	GcpCloudRunJobExecution rac.ResourceAttributeConfig `river:"gcp.cloud_run.job.execution,block,optional"`
	GcpCloudRunJobTaskIndex rac.ResourceAttributeConfig `river:"gcp.cloud_run.job.task_index,block,optional"`
	GcpGceInstanceHostname  rac.ResourceAttributeConfig `river:"gcp.gce.instance.hostname,block,optional"`
	GcpGceInstanceName      rac.ResourceAttributeConfig `river:"gcp.gce.instance.name,block,optional"`
	HostID                  rac.ResourceAttributeConfig `river:"host.id,block,optional"`
	HostName                rac.ResourceAttributeConfig `river:"host.name,block,optional"`
	HostType                rac.ResourceAttributeConfig `river:"host.type,block,optional"`
	K8sClusterName          rac.ResourceAttributeConfig `river:"k8s.cluster.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"cloud.account.id":             r.CloudAccountID.Convert(),
		"cloud.availability_zone":      r.CloudAvailabilityZone.Convert(),
		"cloud.platform":               r.CloudPlatform.Convert(),
		"cloud.provider":               r.CloudProvider.Convert(),
		"cloud.region":                 r.CloudRegion.Convert(),
		"faas.id":                      r.FaasID.Convert(),
		"faas.instance":                r.FaasInstance.Convert(),
		"faas.name":                    r.FaasName.Convert(),
		"faas.version":                 r.FaasVersion.Convert(),
		"gcp.cloud_run.job.execution":  r.GcpCloudRunJobExecution.Convert(),
		"gcp.cloud_run.job.task_index": r.GcpCloudRunJobTaskIndex.Convert(),
		"gcp.gce.instance.hostname":    r.GcpGceInstanceHostname.Convert(),
		"gcp.gce.instance.name":        r.GcpGceInstanceName.Convert(),
		"host.id":                      r.HostID.Convert(),
		"host.name":                    r.HostName.Convert(),
		"host.type":                    r.HostType.Convert(),
		"k8s.cluster.name":             r.K8sClusterName.Convert(),
	}
}
