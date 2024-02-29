package azure

import (
	"fmt"
	"time"

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	common "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/azure"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.azure",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Environment     string           `river:"environment,attr,optional"`
	Port            int              `river:"port,attr,optional"`
	SubscriptionID  string           `river:"subscription_id,attr,optional"`
	OAuth           *OAuth           `river:"oauth,block,optional"`
	ManagedIdentity *ManagedIdentity `river:"managed_identity,block,optional"`
	RefreshInterval time.Duration    `river:"refresh_interval,attr,optional"`
	ResourceGroup   string           `river:"resource_group,attr,optional"`

	ProxyConfig     *config.ProxyConfig `river:",squash"`
	FollowRedirects bool                `river:"follow_redirects,attr,optional"`
	EnableHTTP2     bool                `river:"enable_http2,attr,optional"`
	TLSConfig       config.TLSConfig    `river:"tls_config,block,optional"`
}

type OAuth struct {
	ClientID     string            `river:"client_id,attr"`
	TenantID     string            `river:"tenant_id,attr"`
	ClientSecret rivertypes.Secret `river:"client_secret,attr"`
}

type ManagedIdentity struct {
	ClientID string `river:"client_id,attr"`
}

var DefaultArguments = Arguments{
	Environment:     azure.PublicCloud.Name,
	Port:            80,
	RefreshInterval: 5 * time.Minute,
	FollowRedirects: true,
	EnableHTTP2:     true,
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	if a.OAuth == nil && a.ManagedIdentity == nil || a.OAuth != nil && a.ManagedIdentity != nil {
		return fmt.Errorf("exactly one of oauth or managed_identity must be specified")
	}

	if err := a.TLSConfig.Validate(); err != nil {
		return err
	}

	return a.ProxyConfig.Validate()
}

func (a *Arguments) Convert() *prom_discovery.SDConfig {
	var (
		authMethod   string
		clientID     string
		tenantID     string
		clientSecret common.Secret
	)
	if a.OAuth != nil {
		authMethod = "OAuth"
		clientID = a.OAuth.ClientID
		tenantID = a.OAuth.TenantID
		clientSecret = common.Secret(a.OAuth.ClientSecret)
	} else {
		authMethod = "ManagedIdentity"
		clientID = a.ManagedIdentity.ClientID
	}

	httpClientConfig := config.DefaultHTTPClientConfig
	httpClientConfig.FollowRedirects = a.FollowRedirects
	httpClientConfig.EnableHTTP2 = a.EnableHTTP2
	httpClientConfig.TLSConfig = a.TLSConfig
	httpClientConfig.ProxyConfig = a.ProxyConfig

	return &prom_discovery.SDConfig{
		Environment:          a.Environment,
		Port:                 a.Port,
		SubscriptionID:       a.SubscriptionID,
		TenantID:             tenantID,
		ClientID:             clientID,
		ClientSecret:         clientSecret,
		RefreshInterval:      model.Duration(a.RefreshInterval),
		AuthenticationMethod: authMethod,
		ResourceGroup:        a.ResourceGroup,
		HTTPClientConfig:     *httpClientConfig.Convert(),
	}
}

// New returns a new instance of a discovery.azure component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger), nil
	})
}
