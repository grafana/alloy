package triton

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/triton"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.triton",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Account         string           `river:"account,attr"`
	Role            string           `river:"role,attr,optional"`
	DNSSuffix       string           `river:"dns_suffix,attr"`
	Endpoint        string           `river:"endpoint,attr"`
	Groups          []string         `river:"groups,attr,optional"`
	Port            int              `river:"port,attr,optional"`
	RefreshInterval time.Duration    `river:"refresh_interval,attr,optional"`
	Version         int              `river:"version,attr,optional"`
	TLSConfig       config.TLSConfig `river:"tls_config,block,optional"`
}

var DefaultArguments = Arguments{
	Role:            "container",
	Port:            9163,
	RefreshInterval: 60 * time.Second,
	Version:         1,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	if args.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0")
	}

	return nil
}

func (args *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		Account:         args.Account,
		Role:            args.Role,
		DNSSuffix:       args.DNSSuffix,
		Endpoint:        args.Endpoint,
		Groups:          args.Groups,
		Port:            args.Port,
		RefreshInterval: model.Duration(args.RefreshInterval),
		TLSConfig:       *args.TLSConfig.Convert(),
		Version:         args.Version,
	}
}

// New returns a new instance of a discovery.triton component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.New(opts.Logger, newArgs.Convert())
	})
}
