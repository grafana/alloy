package heroku

import (
	rac "github.com/grafana/agent/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/river"
)

const Name = "heroku"

type Config struct {
	ResourceAttributes ResourceAttributesConfig `river:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudProvider:                  rac.ResourceAttributeConfig{Enabled: true},
		HerokuAppID:                    rac.ResourceAttributeConfig{Enabled: true},
		HerokuDynoID:                   rac.ResourceAttributeConfig{Enabled: true},
		HerokuReleaseCommit:            rac.ResourceAttributeConfig{Enabled: true},
		HerokuReleaseCreationTimestamp: rac.ResourceAttributeConfig{Enabled: true},
		ServiceInstanceID:              rac.ResourceAttributeConfig{Enabled: true},
		ServiceName:                    rac.ResourceAttributeConfig{Enabled: true},
		ServiceVersion:                 rac.ResourceAttributeConfig{Enabled: true},
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

// ResourceAttributesConfig provides config for heroku resource attributes.
type ResourceAttributesConfig struct {
	CloudProvider                  rac.ResourceAttributeConfig `river:"cloud.provider,block,optional"`
	HerokuAppID                    rac.ResourceAttributeConfig `river:"heroku.app.id,block,optional"`
	HerokuDynoID                   rac.ResourceAttributeConfig `river:"heroku.dyno.id,block,optional"`
	HerokuReleaseCommit            rac.ResourceAttributeConfig `river:"heroku.release.commit,block,optional"`
	HerokuReleaseCreationTimestamp rac.ResourceAttributeConfig `river:"heroku.release.creation_timestamp,block,optional"`
	ServiceInstanceID              rac.ResourceAttributeConfig `river:"service.instance.id,block,optional"`
	ServiceName                    rac.ResourceAttributeConfig `river:"service.name,block,optional"`
	ServiceVersion                 rac.ResourceAttributeConfig `river:"service.version,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]interface{} {
	return map[string]interface{}{
		"cloud.provider":                    r.CloudProvider.Convert(),
		"heroku.app.id":                     r.HerokuAppID.Convert(),
		"heroku.dyno.id":                    r.HerokuDynoID.Convert(),
		"heroku.release.commit":             r.HerokuReleaseCommit.Convert(),
		"heroku.release.creation_timestamp": r.HerokuReleaseCreationTimestamp.Convert(),
		"service.instance.id":               r.ServiceInstanceID.Convert(),
		"service.name":                      r.ServiceName.Convert(),
		"service.version":                   r.ServiceVersion.Convert(),
	}
}
