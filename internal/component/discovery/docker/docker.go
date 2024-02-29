// Package docker implements the discovery.docker component.
package docker

import (
	"fmt"
	"net/url"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/moby"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.docker",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the discovery.docker component.
type Arguments struct {
	Host               string                  `river:"host,attr"`
	Port               int                     `river:"port,attr,optional"`
	HostNetworkingHost string                  `river:"host_networking_host,attr,optional"`
	RefreshInterval    time.Duration           `river:"refresh_interval,attr,optional"`
	Filters            []Filter                `river:"filter,block,optional"`
	HTTPClientConfig   config.HTTPClientConfig `river:",squash"`
}

// Filter is used to limit the discovery process to a subset of available
// resources.
type Filter struct {
	Name   string   `river:"name,attr"`
	Values []string `river:"values,attr"`
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
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
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

// Convert converts Arguments to the upstream Prometheus SD type.
func (args Arguments) Convert() moby.DockerSDConfig {
	filters := make([]moby.Filter, len(args.Filters))
	for i, filter := range args.Filters {
		filters[i] = filter.Convert()
	}

	return moby.DockerSDConfig{
		HTTPClientConfig: *args.HTTPClientConfig.Convert(),

		Host:               args.Host,
		Port:               args.Port,
		Filters:            filters,
		HostNetworkingHost: args.HostNetworkingHost,

		RefreshInterval: model.Duration(args.RefreshInterval),
	}
}

// New returns a new instance of a discovery.docker component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		conf := args.(Arguments).Convert()
		return moby.NewDockerDiscovery(&conf, opts.Logger)
	})
}
