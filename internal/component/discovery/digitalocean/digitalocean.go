package digitalocean

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/digitalocean"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.digitalocean",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	RefreshInterval time.Duration `alloy:"refresh_interval,attr,optional"`
	Port            int           `alloy:"port,attr,optional"`

	BearerToken     alloytypes.Secret `alloy:"bearer_token,attr,optional"`
	BearerTokenFile string            `alloy:"bearer_token_file,attr,optional"`

	ProxyConfig     *config.ProxyConfig `alloy:",squash"`
	FollowRedirects bool                `alloy:"follow_redirects,attr,optional"`
	EnableHTTP2     bool                `alloy:"enable_http2,attr,optional"`
	HTTPHeaders     *config.Headers     `alloy:",squash"`
}

var DefaultArguments = Arguments{
	Port:            80,
	RefreshInterval: time.Minute,
	FollowRedirects: true,
	EnableHTTP2:     true,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
//
// Validate validates the arguments. Specifically, it checks that a BearerToken or
// BearerTokenFile is specified, as the DigitalOcean API requires a Bearer Token for
// authentication.
func (a *Arguments) Validate() error {
	if (a.BearerToken == "" && a.BearerTokenFile == "") ||
		(len(a.BearerToken) > 0 && len(a.BearerTokenFile) > 0) {

		return fmt.Errorf("exactly one of bearer_token or bearer_token_file must be specified")
	}

	if err := a.HTTPHeaders.Validate(); err != nil {
		return err
	}

	return a.ProxyConfig.Validate()
}

func (a Arguments) Convert() discovery.DiscovererConfig {
	httpClientConfig := config.DefaultHTTPClientConfig
	httpClientConfig.BearerToken = a.BearerToken
	httpClientConfig.BearerTokenFile = a.BearerTokenFile
	httpClientConfig.FollowRedirects = a.FollowRedirects
	httpClientConfig.EnableHTTP2 = a.EnableHTTP2
	httpClientConfig.ProxyConfig = a.ProxyConfig
	httpClientConfig.HTTPHeaders = a.HTTPHeaders

	return &prom_discovery.SDConfig{
		RefreshInterval:  model.Duration(a.RefreshInterval),
		Port:             a.Port,
		HTTPClientConfig: *httpClientConfig.Convert(),
	}
}
