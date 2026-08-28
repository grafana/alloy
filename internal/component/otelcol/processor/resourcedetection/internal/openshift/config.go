package openshift

import (
	"github.com/grafana/alloy/internal/component/otelcol"
	rac "github.com/grafana/alloy/internal/component/otelcol/processor/resourcedetection/internal/resource_attribute_config"
	"github.com/grafana/alloy/syntax"
)

const Name = "openshift"

// Config can contain user-specified inputs to overwrite default values.
// See `openshift.go#NewDetector` for more information.
type Config struct {
	// Address is the address of the openshift api server
	Address string `alloy:"address,attr,optional"`

	// Token is used to identify against the openshift api server
	Token string `alloy:"token,attr,optional"`

	// TLSSettings contains TLS configurations that are specific to client
	// connection used to communicate with the Openshift API.
	TLSSettings otelcol.TLSClientArguments `alloy:"tls,block,optional"`

	ResourceAttributes ResourceAttributesConfig `alloy:"resource_attributes,block,optional"`
}

// DefaultArguments holds default settings for Config.
var DefaultArguments = Config{
	ResourceAttributes: ResourceAttributesConfig{
		CloudPlatform:  rac.ResourceAttributeConfig{Enabled: true},
		CloudProvider:  rac.ResourceAttributeConfig{Enabled: true},
		CloudRegion:    rac.ResourceAttributeConfig{Enabled: true},
		K8sClusterName: rac.ResourceAttributeConfig{Enabled: true},
	},
}

var _ syntax.Defaulter = (*Config)(nil)

// SetToDefault implements syntax.Defaulter.
func (args *Config) SetToDefault() {
	*args = DefaultArguments
}

func (args Config) Convert() map[string]any {
	return map[string]any{
		"address":             args.Address,
		"token":               args.Token,
		"tls":                 args.TLSSettings.Convert(),
		"resource_attributes": args.ResourceAttributes.Convert(),
	}
}

// ResourceAttributesConfig provides config for openshift resource attributes.
type ResourceAttributesConfig struct {
	CloudPlatform  rac.ResourceAttributeConfig `alloy:"cloud.platform,block,optional"`
	CloudProvider  rac.ResourceAttributeConfig `alloy:"cloud.provider,block,optional"`
	CloudRegion    rac.ResourceAttributeConfig `alloy:"cloud.region,block,optional"`
	K8sClusterName rac.ResourceAttributeConfig `alloy:"k8s.cluster.name,block,optional"`
}

func (r ResourceAttributesConfig) Convert() map[string]any {
	return map[string]any{
		"cloud.platform":   r.CloudPlatform.Convert(),
		"cloud.provider":   r.CloudProvider.Convert(),
		"cloud.region":     r.CloudRegion.Convert(),
		"k8s.cluster.name": r.K8sClusterName.Convert(),
	}
}
