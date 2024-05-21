package consul

import (
	"fmt"
	"time"

	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/consul"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.consul",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server       string            `alloy:"server,attr,optional"`
	Token        alloytypes.Secret `alloy:"token,attr,optional"`
	Datacenter   string            `alloy:"datacenter,attr,optional"`
	Namespace    string            `alloy:"namespace,attr,optional"`
	Partition    string            `alloy:"partition,attr,optional"`
	TagSeparator string            `alloy:"tag_separator,attr,optional"`
	Scheme       string            `alloy:"scheme,attr,optional"`
	Username     string            `alloy:"username,attr,optional"`
	Password     alloytypes.Secret `alloy:"password,attr,optional"`
	AllowStale   bool              `alloy:"allow_stale,attr,optional"`
	Services     []string          `alloy:"services,attr,optional"`
	ServiceTags  []string          `alloy:"tags,attr,optional"`
	NodeMeta     map[string]string `alloy:"node_meta,attr,optional"`

	RefreshInterval  time.Duration           `alloy:"refresh_interval,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

var DefaultArguments = Arguments{
	Server:           "localhost:8500",
	TagSeparator:     ",",
	Scheme:           "http",
	AllowStale:       true,
	RefreshInterval:  30 * time.Second,
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

func (args Arguments) Convert() discovery.DiscovererConfig {
	httpClient := &args.HTTPClientConfig

	return &prom_discovery.SDConfig{
		RefreshInterval:  model.Duration(args.RefreshInterval),
		HTTPClientConfig: *httpClient.Convert(),
		Server:           args.Server,
		Token:            config_util.Secret(args.Token),
		Datacenter:       args.Datacenter,
		Namespace:        args.Namespace,
		Partition:        args.Partition,
		TagSeparator:     args.TagSeparator,
		Scheme:           args.Scheme,
		Username:         args.Username,
		Password:         config_util.Secret(args.Password),
		AllowStale:       args.AllowStale,
		Services:         args.Services,
		ServiceTags:      args.ServiceTags,
		NodeMeta:         args.NodeMeta,
	}
}
