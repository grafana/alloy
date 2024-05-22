package eureka

import (
	"fmt"
	"net/url"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/eureka"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.eureka",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server          string        `alloy:"server,attr"`
	RefreshInterval time.Duration `alloy:"refresh_interval,attr,optional"`

	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
}

var DefaultArguments = Arguments{
	RefreshInterval:  30 * time.Second,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
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

func (a Arguments) Convert() discovery.DiscovererConfig {
	return &prom_discovery.SDConfig{
		Server:           a.Server,
		HTTPClientConfig: *a.HTTPClientConfig.Convert(),
		RefreshInterval:  model.Duration(a.RefreshInterval),
	}
}
