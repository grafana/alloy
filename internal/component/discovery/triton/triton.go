package triton

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/triton"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.triton",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Account         string           `alloy:"account,attr"`
	Role            string           `alloy:"role,attr,optional"`
	DNSSuffix       string           `alloy:"dns_suffix,attr"`
	Endpoint        string           `alloy:"endpoint,attr"`
	Groups          []string         `alloy:"groups,attr,optional"`
	Port            int              `alloy:"port,attr,optional"`
	RefreshInterval time.Duration    `alloy:"refresh_interval,attr,optional"`
	Version         int              `alloy:"version,attr,optional"`
	TLSConfig       config.TLSConfig `alloy:"tls_config,block,optional"`
}

var DefaultArguments = Arguments{
	Role:            "container",
	Port:            9163,
	RefreshInterval: 60 * time.Second,
	Version:         1,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0")
	}

	return nil
}

func (args Arguments) Convert() discovery.DiscovererConfig {
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
