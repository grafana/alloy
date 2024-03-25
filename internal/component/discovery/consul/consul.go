package consul

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/consul"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.consul",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
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

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	if args.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0")
	}

	return args.HTTPClientConfig.Validate()
}

func (args *Arguments) Convert() *prom_discovery.SDConfig {
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

// New returns a new instance of a discovery.consul component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
