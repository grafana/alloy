package ovhcloud

import (
	"fmt"
	"time"

	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/ovhcloud"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.ovhcloud",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

// Arguments configure the discovery.ovhcloud component.
type Arguments struct {
	Endpoint          string            `alloy:"endpoint,attr,optional"`
	ApplicationKey    string            `alloy:"application_key,attr"`
	ApplicationSecret alloytypes.Secret `alloy:"application_secret,attr"`
	ConsumerKey       alloytypes.Secret `alloy:"consumer_key,attr"`
	RefreshInterval   time.Duration     `alloy:"refresh_interval,attr,optional"`
	Service           string            `alloy:"service,attr"`
}

// DefaultArguments is used to initialize default values for Arguments.
var DefaultArguments = Arguments{
	Endpoint:        "ovh-eu",
	RefreshInterval: 60 * time.Second,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}

	if args.ApplicationKey == "" {
		return fmt.Errorf("application_key cannot be empty")
	}

	if args.ApplicationSecret == "" {
		return fmt.Errorf("application_secret cannot be empty")
	}

	if args.ConsumerKey == "" {
		return fmt.Errorf("consumer_key cannot be empty")
	}

	switch args.Service {
	case "dedicated_server", "vps":
		// Valid value - do nothing.
	default:
		return fmt.Errorf("unknown service: %v", args.Service)
	}

	return nil
}

func (args Arguments) Convert() discovery.DiscovererConfig {
	return &prom_discovery.SDConfig{
		Endpoint:          args.Endpoint,
		ApplicationKey:    args.ApplicationKey,
		ApplicationSecret: config.Secret(args.ApplicationSecret),
		ConsumerKey:       config.Secret(args.ConsumerKey),
		RefreshInterval:   model.Duration(args.RefreshInterval),
		Service:           args.Service,
	}
}
