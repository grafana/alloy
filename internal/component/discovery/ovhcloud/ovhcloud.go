package ovhcloud

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/ovhcloud"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.ovhcloud",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configure the discovery.ovhcloud component.
type Arguments struct {
	Endpoint          string            `river:"endpoint,attr,optional"`
	ApplicationKey    string            `river:"application_key,attr"`
	ApplicationSecret rivertypes.Secret `river:"application_secret,attr"`
	ConsumerKey       rivertypes.Secret `river:"consumer_key,attr"`
	RefreshInterval   time.Duration     `river:"refresh_interval,attr,optional"`
	Service           string            `river:"service,attr"`
}

// DefaultArguments is used to initialize default values for Arguments.
var DefaultArguments = Arguments{
	Endpoint:        "ovh-eu",
	RefreshInterval: 60 * time.Second,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
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

// Convert returns the upstream configuration struct.
func (args *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		Endpoint:          args.Endpoint,
		ApplicationKey:    args.ApplicationKey,
		ApplicationSecret: config.Secret(args.ApplicationSecret),
		ConsumerKey:       config.Secret(args.ConsumerKey),
		RefreshInterval:   model.Duration(args.RefreshInterval),
		Service:           args.Service,
	}
}

// New returns a new instance of a discovery.ovhcloud component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
