package nomad

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/nomad"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.nomad",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	AllowStale       bool                    `river:"allow_stale,attr,optional"`
	HTTPClientConfig config.HTTPClientConfig `river:",squash"`
	Namespace        string                  `river:"namespace,attr,optional"`
	RefreshInterval  time.Duration           `river:"refresh_interval,attr,optional"`
	Region           string                  `river:"region,attr,optional"`
	Server           string                  `river:"server,attr,optional"`
	TagSeparator     string                  `river:"tag_separator,attr,optional"`
}

var DefaultArguments = Arguments{
	AllowStale:       true,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
	Namespace:        "default",
	RefreshInterval:  60 * time.Second,
	Region:           "global",
	Server:           "http://localhost:4646",
	TagSeparator:     ",",
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	if strings.TrimSpace(a.Server) == "" {
		return fmt.Errorf("nomad SD configuration requires a server address")
	}
	return a.HTTPClientConfig.Validate()
}

func (a *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		AllowStale:       a.AllowStale,
		HTTPClientConfig: *a.HTTPClientConfig.Convert(),
		Namespace:        a.Namespace,
		RefreshInterval:  model.Duration(a.RefreshInterval),
		Region:           a.Region,
		Server:           a.Server,
		TagSeparator:     a.TagSeparator,
	}
}

// New returns a new instance of a discovery.azure component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
