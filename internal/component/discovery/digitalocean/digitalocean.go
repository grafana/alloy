package digitalocean

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/digitalocean"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.digitalocean",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	RefreshInterval time.Duration `river:"refresh_interval,attr,optional"`
	Port            int           `river:"port,attr,optional"`

	BearerToken     rivertypes.Secret `river:"bearer_token,attr,optional"`
	BearerTokenFile string            `river:"bearer_token_file,attr,optional"`

	ProxyConfig     *config.ProxyConfig `river:",squash"`
	FollowRedirects bool                `river:"follow_redirects,attr,optional"`
	EnableHTTP2     bool                `river:"enable_http2,attr,optional"`
}

var DefaultArguments = Arguments{
	Port:            80,
	RefreshInterval: time.Minute,
	FollowRedirects: true,
	EnableHTTP2:     true,
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
//
// Validate validates the arguments. Specifically, it checks that a BearerToken or
// BearerTokenFile is specified, as the DigitalOcean API requires a Bearer Token for
// authentication.
func (a *Arguments) Validate() error {
	if (a.BearerToken == "" && a.BearerTokenFile == "") ||
		(len(a.BearerToken) > 0 && len(a.BearerTokenFile) > 0) {

		return fmt.Errorf("exactly one of bearer_token or bearer_token_file must be specified")
	}

	return a.ProxyConfig.Validate()
}

func (a *Arguments) Convert() *prom_discovery.SDConfig {
	httpClientConfig := config.DefaultHTTPClientConfig
	httpClientConfig.BearerToken = a.BearerToken
	httpClientConfig.BearerTokenFile = a.BearerTokenFile
	httpClientConfig.FollowRedirects = a.FollowRedirects
	httpClientConfig.EnableHTTP2 = a.EnableHTTP2
	httpClientConfig.ProxyConfig = a.ProxyConfig

	return &prom_discovery.SDConfig{
		RefreshInterval:  model.Duration(a.RefreshInterval),
		Port:             a.Port,
		HTTPClientConfig: *httpClientConfig.Convert(),
	}
}

// New returns a new instance of a discovery.digitalocean component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
