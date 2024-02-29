package marathon

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/marathon"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.marathon",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Servers          []string                `river:"servers,attr"`
	RefreshInterval  time.Duration           `river:"refresh_interval,attr,optional"`
	AuthToken        rivertypes.Secret       `river:"auth_token,attr,optional"`
	AuthTokenFile    string                  `river:"auth_token_file,attr,optional"`
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
	if a.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be greater than 0")
	}
	if len(a.Servers) == 0 {
		return fmt.Errorf("at least one Marathon server must be specified")
	}
	if len(a.AuthToken) > 0 && len(a.AuthTokenFile) > 0 {
		return fmt.Errorf("at most one of auth_token and auth_token_file must be configured")
	}
	if len(a.AuthToken) > 0 || len(a.AuthTokenFile) > 0 {
		switch {
		case a.HTTPClientConfig.BasicAuth != nil:
			return fmt.Errorf("at most one of basic_auth, auth_token & auth_token_file must be configured")
		case len(a.HTTPClientConfig.BearerToken) > 0 || len(a.HTTPClientConfig.BearerTokenFile) > 0:
			return fmt.Errorf("at most one of bearer_token, bearer_token_file, auth_token & auth_token_file must be configured")
		case a.HTTPClientConfig.Authorization != nil:
			return fmt.Errorf("at most one of auth_token, auth_token_file & authorization must be configured")
		}
	}
	return a.HTTPClientConfig.Validate()
}

// Convert converts Arguments into the SDConfig type.
func (a *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		Servers:          a.Servers,
		RefreshInterval:  model.Duration(a.RefreshInterval),
		AuthToken:        promcfg.Secret(a.AuthToken),
		AuthTokenFile:    a.AuthTokenFile,
		HTTPClientConfig: *a.HTTPClientConfig.Convert(),
	}
}

// New returns a new instance of discovery.marathon component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(*newArgs.Convert(), opts.Logger)
	})
}
