package nerve

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/zookeeper"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.nerve",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configure the discovery.nerve component.
type Arguments struct {
	Servers []string      `alloy:"servers,attr"`
	Paths   []string      `alloy:"paths,attr"`
	Timeout time.Duration `alloy:"timeout,attr,optional"`
}

// DefaultArguments is used to initialize default values for Arguments.
var DefaultArguments = Arguments{
	Timeout: 10 * time.Second,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}

// Convert returns the upstream configuration struct.
func (args *Arguments) Convert() *prom_discovery.NerveSDConfig {
	return &prom_discovery.NerveSDConfig{
		Servers: args.Servers,
		Paths:   args.Paths,
		Timeout: model.Duration(args.Timeout),
	}
}

// New returns a new instance of a discovery.nerve component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		newArgs := args.(Arguments)
		return newArgs.Convert(), nil
	})
}
