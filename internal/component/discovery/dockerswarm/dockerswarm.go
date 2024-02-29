package dockerswarm

import (
	"fmt"
	"net/url"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/moby"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.dockerswarm",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Host             string                  `river:"host,attr"`
	Role             string                  `river:"role,attr"`
	Port             int                     `river:"port,attr,optional"`
	Filters          []Filter                `river:"filter,block,optional"`
	RefreshInterval  time.Duration           `river:"refresh_interval,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `river:",squash"`
}

type Filter struct {
	Name   string   `river:"name,attr"`
	Values []string `river:"values,attr"`
}

var DefaultArguments = Arguments{
	RefreshInterval:  60 * time.Second,
	Port:             80,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	if _, err := url.Parse(a.Host); err != nil {
		return err
	}
	if a.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0")
	}
	switch a.Role {
	case "services", "nodes", "tasks":
	default:
		return fmt.Errorf("invalid role %s, expected tasks, services, or nodes", a.Role)
	}
	return a.HTTPClientConfig.Validate()
}

// Convert converts Arguments into the SDConfig type.
func (a *Arguments) Convert() *prom_discovery.DockerSwarmSDConfig {
	return &prom_discovery.DockerSwarmSDConfig{
		Host:             a.Host,
		Role:             a.Role,
		Port:             a.Port,
		Filters:          convertFilters(a.Filters),
		RefreshInterval:  model.Duration(a.RefreshInterval),
		HTTPClientConfig: *a.HTTPClientConfig.Convert(),
	}
}

func convertFilters(filters []Filter) []prom_discovery.Filter {
	promFilters := make([]prom_discovery.Filter, len(filters))
	for i, filter := range filters {
		promFilters[i] = filter.convert()
	}
	return promFilters
}

func (f *Filter) convert() prom_discovery.Filter {
	values := make([]string, len(f.Values))
	copy(values, f.Values)

	return prom_discovery.Filter{
		Name:   f.Name,
		Values: values,
	}
}

// New returns a new instance of discovery.dockerswarm component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
