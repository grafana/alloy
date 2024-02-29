package puppetdb

import (
	"fmt"
	"net/url"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/puppetdb"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.puppetdb",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	HTTPClientConfig  config.HTTPClientConfig `river:",squash"`
	RefreshInterval   time.Duration           `river:"refresh_interval,attr,optional"`
	URL               string                  `river:"url,attr"`
	Query             string                  `river:"query,attr"`
	IncludeParameters bool                    `river:"include_parameters,attr,optional"`
	Port              int                     `river:"port,attr,optional"`
}

var DefaultArguments = Arguments{
	RefreshInterval:  60 * time.Second,
	Port:             80,
	HTTPClientConfig: config.DefaultHTTPClientConfig,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	parsedURL, err := url.Parse(args.URL)
	if err != nil {
		return err
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be 'http' or 'https'")
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("host is missing in URL")
	}
	return args.HTTPClientConfig.Validate()
}

func (args *Arguments) Convert() *prom_discovery.SDConfig {
	httpClient := &args.HTTPClientConfig

	return &prom_discovery.SDConfig{
		URL:               args.URL,
		Query:             args.Query,
		IncludeParameters: args.IncludeParameters,
		Port:              args.Port,
		RefreshInterval:   model.Duration(args.RefreshInterval),
		HTTPClientConfig:  *httpClient.Convert(),
	}
}

// New returns a new instance of a discovery.puppetdb component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
