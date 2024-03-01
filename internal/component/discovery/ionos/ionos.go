package ionos

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/ionos"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.ionos",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	DatacenterID     string                  `river:"datacenter_id,attr"`
	HTTPClientConfig config.HTTPClientConfig `river:",squash"`
	RefreshInterval  time.Duration           `river:"refresh_interval,attr,optional"`
	Port             int                     `river:"port,attr,optional"`
}

var DefaultArguments = Arguments{
	HTTPClientConfig: config.DefaultHTTPClientConfig,
	RefreshInterval:  60 * time.Second,
	Port:             80,
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	if a.DatacenterID == "" {
		return fmt.Errorf("datacenter_id can't be empty")
	}
	if a.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0")
	}
	return a.HTTPClientConfig.Validate()
}

// Convert converts Arguments into the SDConfig type.
func (a *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		DatacenterID:     a.DatacenterID,
		Port:             a.Port,
		RefreshInterval:  model.Duration(a.RefreshInterval),
		HTTPClientConfig: *a.HTTPClientConfig.Convert(),
	}
}

// New returns a new instance of a discovery.ionos component,
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
