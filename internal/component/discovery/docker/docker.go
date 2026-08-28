// Package docker implements the discovery.docker component.
package docker

import (
	"fmt"
	"net/url"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/moby"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.docker",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

// Arguments configures the discovery.docker component.
type Arguments struct {
	Host               string                  `alloy:"host,attr"`
	Port               int                     `alloy:"port,attr,optional"`
	HostNetworkingHost string                  `alloy:"host_networking_host,attr,optional"`
	RefreshInterval    time.Duration           `alloy:"refresh_interval,attr,optional"`
	MatchFirstNetwork  bool                    `alloy:"match_first_network,attr,optional"`
	Filters            []Filter                `alloy:"filter,block,optional"`
	HTTPClientConfig   config.HTTPClientConfig `alloy:",squash"`
}

// Filter is used to limit the discovery process to a subset of available
// resources.
type Filter struct {
	Name   string   `alloy:"name,attr"`
	Values []string `alloy:"values,attr"`
}

// Convert converts a Filter to the upstream Prometheus SD type.
func (f Filter) Convert() moby.Filter {
	return moby.Filter{
		Name:   f.Name,
		Values: f.Values,
	}
}

// DefaultArguments holds default values for Arguments.
var DefaultArguments = Arguments{
	Port:               80,
	HostNetworkingHost: "localhost",
	RefreshInterval:    time.Minute,
	HTTPClientConfig:   config.DefaultHTTPClientConfig,
	MatchFirstNetwork:  true,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Host == "" {
		return fmt.Errorf("host attribute must not be empty")
	} else if _, err := url.Parse(args.Host); err != nil {
		return fmt.Errorf("parsing host attribute: %w", err)
	}

	if args.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0")
	}

	return args.HTTPClientConfig.Validate()
}

func (args Arguments) Convert() discovery.DiscovererConfig {
	filters := make([]moby.Filter, len(args.Filters))
	for i, filter := range args.Filters {
		filters[i] = filter.Convert()
	}

	return &moby.DockerSDConfig{
		HTTPClientConfig: *args.HTTPClientConfig.Convert(),

		Host:               args.Host,
		Port:               args.Port,
		Filters:            filters,
		HostNetworkingHost: args.HostNetworkingHost,
		MatchFirstNetwork:  args.MatchFirstNetwork,

		RefreshInterval: model.Duration(args.RefreshInterval),
	}
}
