package consulagent

import (
	"fmt"
	"time"

	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.consulagent",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server          string            `alloy:"server,attr,optional"`
	Token           alloytypes.Secret `alloy:"token,attr,optional"`
	Datacenter      string            `alloy:"datacenter,attr,optional"`
	TagSeparator    string            `alloy:"tag_separator,attr,optional"`
	Scheme          string            `alloy:"scheme,attr,optional"`
	Username        string            `alloy:"username,attr,optional"`
	Password        alloytypes.Secret `alloy:"password,attr,optional"`
	RefreshInterval time.Duration     `alloy:"refresh_interval,attr,optional"`
	Services        []string          `alloy:"services,attr,optional"`
	ServiceTags     []string          `alloy:"tags,attr,optional"`
	TLSConfig       config.TLSConfig  `alloy:"tls_config,block,optional"`
}

var DefaultArguments = Arguments{
	Server:          "localhost:8500",
	TagSeparator:    ",",
	Scheme:          "http",
	RefreshInterval: 30 * time.Second,
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

	return args.TLSConfig.Validate()
}

func (args Arguments) Convert() discovery.DiscovererConfig {
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
