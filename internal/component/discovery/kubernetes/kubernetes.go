// Package kubernetes implements a discovery.kubernetes component.
package kubernetes

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.kubernetes",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the discovery.kubernetes component.
type Arguments struct {
	APIServer          config.URL              `river:"api_server,attr,optional"`
	Role               string                  `river:"role,attr"`
	KubeConfig         string                  `river:"kubeconfig_file,attr,optional"`
	HTTPClientConfig   config.HTTPClientConfig `river:",squash"`
	NamespaceDiscovery NamespaceDiscovery      `river:"namespaces,block,optional"`
	Selectors          []SelectorConfig        `river:"selectors,block,optional"`
	AttachMetadata     AttachMetadataConfig    `river:"attach_metadata,block,optional"`
}

// DefaultConfig holds defaults for SDConfig.
var DefaultConfig = Arguments{
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultConfig
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	return args.HTTPClientConfig.Validate()
}

// Convert converts Arguments to the Prometheus SD type.
func (args *Arguments) Convert() *promk8s.SDConfig {
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
	IncludeOwnNamespace bool     `river:"own_namespace,attr,optional"`
	Names               []string `river:"names,attr,optional"`
}

func (nd *NamespaceDiscovery) convert() *promk8s.NamespaceDiscovery {
	return &promk8s.NamespaceDiscovery{
		IncludeOwnNamespace: nd.IncludeOwnNamespace,
		Names:               nd.Names,
	}
}

// SelectorConfig configures selectors to filter resources to discover.
type SelectorConfig struct {
	Role  string `river:"role,attr"`
	Label string `river:"label,attr,optional"`
	Field string `river:"field,attr,optional"`
}

func (sc *SelectorConfig) convert() *promk8s.SelectorConfig {
	return &promk8s.SelectorConfig{
		Role:  promk8s.Role(sc.Role),
		Label: sc.Label,
		Field: sc.Field,
	}
}

type AttachMetadataConfig struct {
	Node bool `river:"node,attr,optional"`
}

func (am *AttachMetadataConfig) convert() *promk8s.AttachMetadataConfig {
	return &promk8s.AttachMetadataConfig{
		Node: am.Node,
	}
}

// New returns a new instance of a discovery.kubernetes component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return promk8s.New(opts.Logger, newArgs.Convert())
	})
}
