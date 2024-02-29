package consulagent

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.consulagent",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server          string            `river:"server,attr,optional"`
	Token           rivertypes.Secret `river:"token,attr,optional"`
	Datacenter      string            `river:"datacenter,attr,optional"`
	TagSeparator    string            `river:"tag_separator,attr,optional"`
	Scheme          string            `river:"scheme,attr,optional"`
	Username        string            `river:"username,attr,optional"`
	Password        rivertypes.Secret `river:"password,attr,optional"`
	RefreshInterval time.Duration     `river:"refresh_interval,attr,optional"`
	Services        []string          `river:"services,attr,optional"`
	ServiceTags     []string          `river:"tags,attr,optional"`
	TLSConfig       config.TLSConfig  `river:"tls_config,block,optional"`
}

var DefaultArguments = Arguments{
	Server:          "localhost:8500",
	TagSeparator:    ",",
	Scheme:          "http",
	RefreshInterval: 30 * time.Second,
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

	return args.TLSConfig.Validate()
}

// Convert converts Arguments into the SDConfig type.
func (args *Arguments) Convert() *SDConfig {
	return &SDConfig{
		RefreshInterval: model.Duration(args.RefreshInterval),
		Server:          args.Server,
		Token:           promcfg.Secret(args.Token),
		Datacenter:      args.Datacenter,
		TagSeparator:    args.TagSeparator,
		Scheme:          args.Scheme,
		Username:        args.Username,
		Password:        promcfg.Secret(args.Password),
		Services:        args.Services,
		ServiceTags:     args.ServiceTags,
		TLSConfig:       *args.TLSConfig.Convert(),
	}
}

// New returns a new instance of a discovery.consulagent component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
