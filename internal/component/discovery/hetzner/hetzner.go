package hetzner

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/hetzner"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.hetzner",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Role             string                  `alloy:"role,attr"`
	RefreshInterval  time.Duration           `alloy:"refresh_interval,attr,optional"`
	Port             int                     `alloy:"port,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

var DefaultArguments = Arguments{
	Port:             80,
	RefreshInterval:  60 * time.Second,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	switch args.Role {
	case string(prom_discovery.HetznerRoleRobot), string(prom_discovery.HetznerRoleHcloud):
	default:
		return fmt.Errorf("unknown role %s, must be one of robot or hcloud", args.Role)
	}
	return args.HTTPClientConfig.Validate()
}

func (args *Arguments) Convert() *prom_discovery.SDConfig {
	httpClient := &args.HTTPClientConfig

	cfg := &prom_discovery.SDConfig{
		RefreshInterval:  model.Duration(args.RefreshInterval),
		Port:             args.Port,
		HTTPClientConfig: *httpClient.Convert(),
		Role:             prom_discovery.Role(args.Role),
	}
	return cfg
}

// New returns a new instance of a discovery.hetzner component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		newArgs := args.(Arguments)
		return newArgs.Convert(), nil
	})
}
