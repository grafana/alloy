package eureka

import (
	"fmt"
	"net/url"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/eureka"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.eureka",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server          string        `river:"server,attr"`
	RefreshInterval time.Duration `river:"refresh_interval,attr,optional"`

	HTTPClientConfig config.HTTPClientConfig `river:",squash"`
}

var DefaultArguments = Arguments{
	RefreshInterval:  30 * time.Second,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	url, err := url.Parse(a.Server)
	if err != nil {
		return err
	}
	if len(url.Scheme) == 0 || len(url.Host) == 0 {
		return fmt.Errorf("invalid eureka server URL")
	}
	return a.HTTPClientConfig.Validate()
}

func (a *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		Server:           a.Server,
		HTTPClientConfig: *a.HTTPClientConfig.Convert(),
		RefreshInterval:  model.Duration(a.RefreshInterval),
	}
}

func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
