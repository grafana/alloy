package linode

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/linode"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.linode",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configure the discovery.linode component.
type Arguments struct {
	RefreshInterval  time.Duration           `alloy:"refresh_interval,attr,optional"`
	Port             int                     `alloy:"port,attr,optional"`
	TagSeparator     string                  `alloy:"tag_separator,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

// DefaultArguments is used to initialize default values for Arguments.
var DefaultArguments = Arguments{
	TagSeparator:    ",",
	Port:            80,
	RefreshInterval: 60 * time.Second,

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
	return args.HTTPClientConfig.Validate()
}

// Convert returns the upstream configuration struct.
func (args *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		RefreshInterval:  model.Duration(args.RefreshInterval),
		Port:             args.Port,
		TagSeparator:     args.TagSeparator,
		HTTPClientConfig: *(args.HTTPClientConfig.Convert()),
	}
}

// New returns a new instance of a discovery.linode component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		newArgs := args.(Arguments)
		return newArgs.Convert(), nil
	})
}
