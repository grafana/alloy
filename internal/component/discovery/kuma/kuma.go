package kuma

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/xds"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.kuma",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

// Arguments configure the discovery.kuma component.
type Arguments struct {
	Server          string        `alloy:"server,attr"`
	RefreshInterval time.Duration `alloy:"refresh_interval,attr,optional"`
	FetchTimeout    time.Duration `alloy:"fetch_timeout,attr,optional"`

	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

// DefaultArguments is used to initialize default values for Arguments.
var DefaultArguments = Arguments{
	RefreshInterval: 30 * time.Second,
	FetchTimeout:    2 * time.Minute,

	HTTPClientConfig: config.DefaultHTTPClientConfig,
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
	if args.FetchTimeout <= 0 {
		return fmt.Errorf("fetch_timeout must be greater than 0")
	}

	return args.HTTPClientConfig.Validate()
}

func (args Arguments) Convert() discovery.DiscovererConfig {
	return &prom_discovery.SDConfig{
		Server:          args.Server,
		RefreshInterval: model.Duration(args.RefreshInterval),
		FetchTimeout:    model.Duration(args.FetchTimeout),

		HTTPClientConfig: *(args.HTTPClientConfig.Convert()),
	}
}
