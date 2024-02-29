package ecs

import (
	rac "github.com/grafana/agent/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/river"
)

const Name = "ecs"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		AwsEcsClusterArn:      rac.ResourceAttributeConfig{Enabled: true},
		AwsEcsLaunchtype:      rac.ResourceAttributeConfig{Enabled: true},
		AwsEcsTaskArn:         rac.ResourceAttributeConfig{Enabled: true},
		AwsEcsTaskFamily:      rac.ResourceAttributeConfig{Enabled: true},
		AwsEcsTaskRevision:    rac.ResourceAttributeConfig{Enabled: true},
		AwsLogGroupArns:       rac.ResourceAttributeConfig{Enabled: true},
		AwsLogGroupNames:      rac.ResourceAttributeConfig{Enabled: true},
		AwsLogStreamArns:      rac.ResourceAttributeConfig{Enabled: true},
		AwsLogStreamNames:     rac.ResourceAttributeConfig{Enabled: true},
		CloudAccountID:        rac.ResourceAttributeConfig{Enabled: true},
		CloudAvailabilityZone: rac.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:         rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:         rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:           rac.ResourceAttributeConfig{Enabled: true},
	},
}

var _ river.Defaulter = (*Config)(nil)

// SetToDefault implements river.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args *Config) Convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	return map[string]interface{}{
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config for ecs resource attributes.
type ResourceAttributesConfig struct {
	AwsEcsClusterArn      rac.ResourceAttributeConfig `river:"aws.ecs.cluster.arn,block,optional"`
	AwsEcsLaunchtype      rac.ResourceAttributeConfig `river:"aws.ecs.launchtype,block,optional"`
	AwsEcsTaskArn         rac.ResourceAttributeConfig `river:"aws.ecs.task.arn,block,optional"`
	AwsEcsTaskFamily      rac.ResourceAttributeConfig `river:"aws.ecs.task.family,block,optional"`
	AwsEcsTaskRevision    rac.ResourceAttributeConfig `river:"aws.ecs.task.revision,block,optional"`
	AwsLogGroupArns       rac.ResourceAttributeConfig `river:"aws.log.group.arns,block,optional"`
	AwsLogGroupNames      rac.ResourceAttributeConfig `river:"aws.log.group.names,block,optional"`
	AwsLogStreamArns      rac.ResourceAttributeConfig `river:"aws.log.stream.arns,block,optional"`
	AwsLogStreamNames     rac.ResourceAttributeConfig `river:"aws.log.stream.names,block,optional"`
	CloudAccountID        rac.ResourceAttributeConfig `river:"cloud.account.id,block,optional"`
	CloudAvailabilityZone rac.ResourceAttributeConfig `river:"cloud.availability_zone,block,optional"`
	CloudPlatform         rac.ResourceAttributeConfig `river:"cloud.platform,block,optional"`
	CloudProvider         rac.ResourceAttributeConfig `river:"cloud.provider,block,optional"`
	CloudRegion           rac.ResourceAttributeConfig `river:"cloud.region,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"aws.ecs.cluster.arn":     r.AwsEcsClusterArn.Convert(),
		"aws.ecs.launchtype":      r.AwsEcsLaunchtype.Convert(),
		"aws.ecs.task.arn":        r.AwsEcsTaskArn.Convert(),
		"aws.ecs.task.family":     r.AwsEcsTaskFamily.Convert(),
		"aws.ecs.task.revision":   r.AwsEcsTaskRevision.Convert(),
		"aws.log.group.arns":      r.AwsLogGroupArns.Convert(),
		"aws.log.group.names":     r.AwsLogGroupNames.Convert(),
		"aws.log.stream.arns":     r.AwsLogStreamArns.Convert(),
		"aws.log.stream.names":    r.AwsLogStreamNames.Convert(),
		"cloud.account.id":        r.CloudAccountID.Convert(),
		"cloud.availability_zone": r.CloudAvailabilityZone.Convert(),
		"cloud.platform":          r.CloudPlatform.Convert(),
		"cloud.provider":          r.CloudProvider.Convert(),
		"cloud.region":            r.CloudRegion.Convert(),
	}
}
