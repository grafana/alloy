package http

import (
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/http"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.http",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	HTTPClientConfig config.HTTPClientConfig `alloy:",squash"`
	RefreshInterval  time.Duration           `alloy:"refresh_interval,attr,optional"`
	URL              config.URL              `alloy:"url,attr"`
}

var DefaultArguments = Arguments{
	RefreshInterval:  60 * time.Second,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

func (args *Arguments) UnmarshalAlloy(f func(any) error) error {
	*args = DefaultArguments

	type arguments Arguments
	if err := f((*arguments)(args)); err != nil {
		return err
	}

	return nil
}

func (args Arguments) Convert() discovery.DiscovererConfig {
	cfg := &http.SDConfig{
		HTTPClientConfig: *args.HTTPClientConfig.Convert(),
		URL:              args.URL.String(),
		RefreshInterval:  model.Duration(args.RefreshInterval),
	}
	return cfg
}
