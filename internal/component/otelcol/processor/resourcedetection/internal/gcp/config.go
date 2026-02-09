package gcp

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "gcp"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudAccountID:                   rac.ResourceAttributeConfig{Enabled: true},
		CloudAvailabilityZone:            rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:                    rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:                    rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:                      rac.ResourceAttributeConfig{Enabled: true},
		FaasID:                           rac.ResourceAttributeConfig{Enabled: true},
		FaasInstance:                     rac.ResourceAttributeConfig{Enabled: true},
		FaasName:                         rac.ResourceAttributeConfig{Enabled: true},
		FaasVersion:                      rac.ResourceAttributeConfig{Enabled: true},
		GcpCloudRunJobExecution:          rac.ResourceAttributeConfig{Enabled: true},
		GcpCloudRunJobTaskIndex:          rac.ResourceAttributeConfig{Enabled: true},
		GcpGceInstanceHostname:           rac.ResourceAttributeConfig{Enabled: false},
		GcpGceInstanceName:               rac.ResourceAttributeConfig{Enabled: false},
		GcpGceInstanceGroupManagerName:   rac.ResourceAttributeConfig{Enabled: true},
		GcpGceInstanceGroupManagerRegion: rac.ResourceAttributeConfig{Enabled: true},
		GcpGceInstanceGroupManagerZone:   rac.ResourceAttributeConfig{Enabled: true},
		HostID:                           rac.ResourceAttributeConfig{Enabled: true},
		HostName:                         rac.ResourceAttributeConfig{Enabled: true},
		HostType:                         rac.ResourceAttributeConfig{Enabled: true},
		K8sClusterName:                   rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for gcp resource attributes.
type ResourceAttributesConfig struct {
	CloudAccountID                   rac.ResourceAttributeConfig `alloy:"cloud.account.id,block,optional"`
	CloudAvailabilityZone            rac.ResourceAttributeConfig `alloy:"cloud.availability_zone,block,optional"`
	CloudPlatform                    rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider                    rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion                      rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	FaasID                           rac.ResourceAttributeConfig `alloy:"faas.id,block,optional"`
	FaasInstance                     rac.ResourceAttributeConfig `alloy:"faas.instance,block,optional"`
	FaasName                         rac.ResourceAttributeConfig `alloy:"faas.name,block,optional"`
	FaasVersion                      rac.ResourceAttributeConfig `alloy:"faas.version,block,optional"`
	GcpCloudRunJobExecution          rac.ResourceAttributeConfig `alloy:"gcp.cloud_run.job.execution,block,optional"`
	GcpCloudRunJobTaskIndex          rac.ResourceAttributeConfig `alloy:"gcp.cloud_run.job.task_index,block,optional"`
	GcpGceInstanceHostname           rac.ResourceAttributeConfig `alloy:"gcp.gce.instance.hostname,block,optional"`
	GcpGceInstanceName               rac.ResourceAttributeConfig `alloy:"gcp.gce.instance.name,block,optional"`
	GcpGceInstanceGroupManagerName   rac.ResourceAttributeConfig `alloy:"gcp.gce.instance.group_manager.name,block,optional"`
	GcpGceInstanceGroupManagerRegion rac.ResourceAttributeConfig `alloy:"gcp.gce.instance.group_manager.region,block,optional"`
	GcpGceInstanceGroupManagerZone   rac.ResourceAttributeConfig `alloy:"gcp.gce.instance.group_manager.zone,block,optional"`
	HostID                           rac.ResourceAttributeConfig `alloy:"host.id,block,optional"`
	HostName                         rac.ResourceAttributeConfig `alloy:"host.name,block,optional"`
	HostType                         rac.ResourceAttributeConfig `alloy:"host.type,block,optional"`
	K8sClusterName                   rac.ResourceAttributeConfig `alloy:"k8s.cluster.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"cloud.account.id":                      r.CloudAccountID.Convert(),
		"cloud.availability_zone":               r.CloudAvailabilityZone.Convert(),
		"cloud.platform":                        r.CloudPlatform.Convert(),
		"cloud.provider":                        r.CloudProvider.Convert(),
		"cloud.region":                          r.CloudRegion.Convert(),
		"faas.id":                               r.FaasID.Convert(),
		"faas.instance":                         r.FaasInstance.Convert(),
		"faas.name":                             r.FaasName.Convert(),
		"faas.version":                          r.FaasVersion.Convert(),
		"gcp.cloud_run.job.execution":           r.GcpCloudRunJobExecution.Convert(),
		"gcp.cloud_run.job.task_index":          r.GcpCloudRunJobTaskIndex.Convert(),
		"gcp.gce.instance.hostname":             r.GcpGceInstanceHostname.Convert(),
		"gcp.gce.instance.name":                 r.GcpGceInstanceName.Convert(),
		"gcp.gce.instance.group_manager.name":   r.GcpGceInstanceGroupManagerName.Convert(),
		"gcp.gce.instance.group_manager.region": r.GcpGceInstanceGroupManagerRegion.Convert(),
		"gcp.gce.instance.group_manager.zone":   r.GcpGceInstanceGroupManagerZone.Convert(),
		"host.id":                               r.HostID.Convert(),
		"host.name":                             r.HostName.Convert(),
		"host.type":                             r.HostType.Convert(),
		"k8s.cluster.name":                      r.K8sClusterName.Convert(),
	}
}
