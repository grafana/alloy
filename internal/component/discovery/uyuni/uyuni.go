package uyuni

import (
	"fmt"
	"net/url"
	"time"

	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/uyuni"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.uyuni",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server          string              `alloy:"server,attr"`
	Username        string              `alloy:"username,attr"`
	Password        alloytypes.Secret   `alloy:"password,attr"`
	Entitlement     string              `alloy:"entitlement,attr,optional"`
	Separator       string              `alloy:"separator,attr,optional"`
	RefreshInterval time.Duration       `alloy:"refresh_interval,attr,optional"`
	ProxyConfig     *config.ProxyConfig `alloy:",squash"`
	TLSConfig       config.TLSConfig    `alloy:"tls_config,block,optional"`
	FollowRedirects bool                `alloy:"follow_redirects,attr,optional"`
	EnableHTTP2     bool                `alloy:"enable_http2,attr,optional"`
}

var DefaultArguments = Arguments{
	Entitlement:     "monitoring_entitled",
	Separator:       ",",
	RefreshInterval: 1 * time.Minute,

	EnableHTTP2:     config.DefaultHTTPClientConfig.EnableHTTP2,
	FollowRedirects: config.DefaultHTTPClientConfig.FollowRedirects,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	_, err := url.Parse(a.Server)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	if err = a.TLSConfig.Validate(); err != nil {
		return err
	}

	return a.ProxyConfig.Validate()
}

func (a *Arguments) Convert() *prom_discovery.SDConfig {
	return &prom_discovery.SDConfig{
		Server:          a.Server,
		Username:        a.Username,
		Password:        promcfg.Secret(a.Password),
		Entitlement:     a.Entitlement,
		Separator:       a.Separator,
		RefreshInterval: model.Duration(a.RefreshInterval),

		HTTPClientConfig: promcfg.HTTPClientConfig{
			ProxyConfig:     a.ProxyConfig.Convert(),
			TLSConfig:       *a.TLSConfig.Convert(),
			FollowRedirects: a.FollowRedirects,
			EnableHTTP2:     a.EnableHTTP2,
		},
	}
}

// New returns a new instance of a discovery.uyuni component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.DiscovererConfig, error) {
		newArgs := args.(Arguments)
		return newArgs.Convert(), nil
	})
}
