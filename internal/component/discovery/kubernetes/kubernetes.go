// Package kubernetes implements a discovery.kubernetes component.
package kubernetes

import (
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.kubernetes",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

// Arguments configures the discovery.kubernetes component.
type Arguments struct {
	APIServer          config.URL              `alloy:"api_server,attr,optional"`
	Role               string                  `alloy:"role,attr"`
	KubeConfig         string                  `alloy:"kubeconfig_file,attr,optional"`
	HTTPClientConfig   config.HTTPClientConfig `alloy:",squash"`
	NamespaceDiscovery NamespaceDiscovery      `alloy:"namespaces,block,optional"`
	Selectors          []SelectorConfig        `alloy:"selectors,block,optional"`
	AttachMetadata     AttachMetadataConfig    `alloy:"attach_metadata,block,optional"`
}

// DefaultConfig holds defaults for SDConfig.
var DefaultConfig = Arguments{
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultConfig
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return args.HTTPClientConfig.Validate()
}

func (args Arguments) Convert() discovery.DiscovererConfig {
	selectors := make([]promk8s.SelectorConfig, len(args.Selectors))
	for i, s := range args.Selectors {
		selectors[i] = *s.convert()
	}
	return &promk8s.SDConfig{
		APIServer:          args.APIServer.Convert(),
		Role:               promk8s.Role(args.Role),
		KubeConfig:         args.KubeConfig,
		HTTPClientConfig:   *args.HTTPClientConfig.Convert(),
		NamespaceDiscovery: *args.NamespaceDiscovery.convert(),
		Selectors:          selectors,
		AttachMetadata:     *args.AttachMetadata.convert(),
	}
}

// NamespaceDiscovery configures filtering rules for which namespaces to discover.
type NamespaceDiscovery struct {
	IncludeOwnNamespace bool     `alloy:"own_namespace,attr,optional"`
	Names               []string `alloy:"names,attr,optional"`
}

func (nd *NamespaceDiscovery) convert() *promk8s.NamespaceDiscovery {
	return &promk8s.NamespaceDiscovery{
		IncludeOwnNamespace: nd.IncludeOwnNamespace,
		Names:               nd.Names,
	}
}

// SelectorConfig configures selectors to filter resources to discover.
type SelectorConfig struct {
	Role  string `alloy:"role,attr"`
	Label string `alloy:"label,attr,optional"`
	Field string `alloy:"field,attr,optional"`
}

func (sc *SelectorConfig) convert() *promk8s.SelectorConfig {
	return &promk8s.SelectorConfig{
		Role:  promk8s.Role(sc.Role),
		Label: sc.Label,
		Field: sc.Field,
	}
}

type AttachMetadataConfig struct {
	Node      bool `alloy:"node,attr,optional"`
	Namespace bool `alloy:"namespace,attr,optional"`
}

func (am *AttachMetadataConfig) convert() *promk8s.AttachMetadataConfig {
	return &promk8s.AttachMetadataConfig{
		Node:      am.Node,
		Namespace: am.Namespace,
	}
}
