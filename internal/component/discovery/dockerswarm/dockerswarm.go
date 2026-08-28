package dockerswarm

import (
	"fmt"
	"net/url"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/moby"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.dockerswarm",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Host             string                  `alloy:"host,attr"`
	Role             string                  `alloy:"role,attr"`
	Port             int                     `alloy:"port,attr,optional"`
	Filters          []Filter                `alloy:"filter,block,optional"`
	RefreshInterval  time.Duration           `alloy:"refresh_interval,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

type Filter struct {
	Name   string   `alloy:"name,attr"`
	Values []string `alloy:"values,attr"`
}

var DefaultArguments = Arguments{
	RefreshInterval:  60 * time.Second,
	Port:             80,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
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

func (a Arguments) Convert() discovery.DiscovererConfig {
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
