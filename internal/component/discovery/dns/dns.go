package dns

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/dns"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.dns",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the discovery.dns component.
type Arguments struct {
	Names           []string      `alloy:"names,attr"`
	RefreshInterval time.Duration `alloy:"refresh_interval,attr,optional"`
	Type            string        `alloy:"type,attr,optional"`
	Port            int           `alloy:"port,attr,optional"`
}

var DefaultArguments = Arguments{
	RefreshInterval: 30 * time.Second,
	Type:            "SRV",
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	switch strings.ToUpper(args.Type) {
	case "SRV":
	case "A", "AAAA", "MX":
		if args.Port == 0 {
			return errors.New("a port is required in DNS-SD configs for all record types except SRV")
		}
	default:
		return fmt.Errorf("invalid DNS-SD records type %s", args.Type)
	}
	return nil
}

// Convert converts Arguments to the upstream Prometheus SD type.
func (args Arguments) Convert() dns.SDConfig {
	return dns.SDConfig{
		Names:           args.Names,
		RefreshInterval: model.Duration(args.RefreshInterval),
		Type:            args.Type,
		Port:            args.Port,
	}
}

// New returns a new instance of a discovery.dns component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		conf := args.(Arguments).Convert()
		return &conf, nil
	})
}
