package lambda

import (
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "lambda"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		AwsLogGroupNames:  rac.ResourceAttributeConfig{Enabled: true},
		AwsLogStreamNames: rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:     rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:     rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:       rac.ResourceAttributeConfig{Enabled: true},
		FaasInstance:      rac.ResourceAttributeConfig{Enabled: true},
		FaasMaxMemory:     rac.ResourceAttributeConfig{Enabled: true},
		FaasName:          rac.ResourceAttributeConfig{Enabled: true},
		FaasVersion:       rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for lambda resource attributes.
type ResourceAttributesConfig struct {
	AwsLogGroupNames  rac.ResourceAttributeConfig `alloy:"aws.log.group.names,block,optional"`
	AwsLogStreamNames rac.ResourceAttributeConfig `alloy:"aws.log.stream.names,block,optional"`
	CloudPlatform     rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider     rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion       rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	FaasInstance      rac.ResourceAttributeConfig `alloy:"faas.instance,block,optional"`
	FaasMaxMemory     rac.ResourceAttributeConfig `alloy:"faas.max_memory,block,optional"`
	FaasName          rac.ResourceAttributeConfig `alloy:"faas.name,block,optional"`
	FaasVersion       rac.ResourceAttributeConfig `alloy:"faas.version,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"aws.log.group.names":  r.AwsLogGroupNames.Convert(),
		"aws.log.stream.names": r.AwsLogStreamNames.Convert(),
		"cloud.platform":       r.CloudPlatform.Convert(),
		"cloud.provider":       r.CloudProvider.Convert(),
		"cloud.region":         r.CloudRegion.Convert(),
		"faas.instance":        r.FaasInstance.Convert(),
		"faas.max_memory":      r.FaasMaxMemory.Convert(),
		"faas.name":            r.FaasName.Convert(),
		"faas.version":         r.FaasVersion.Convert(),
	}
}
