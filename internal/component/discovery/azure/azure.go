package azure

import (
	"fmt"
	"time"

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	common "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/azure"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.azure",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Environment     string           `alloy:"environment,attr,optional"`
	Port            int              `alloy:"port,attr,optional"`
	SubscriptionID  string           `alloy:"subscription_id,attr,optional"`
	OAuth           *OAuth           `alloy:"oauth,block,optional"`
	ManagedIdentity *ManagedIdentity `alloy:"managed_identity,block,optional"`
	RefreshInterval time.Duration    `alloy:"refresh_interval,attr,optional"`
	ResourceGroup   string           `alloy:"resource_group,attr,optional"`

	ProxyConfig     *config.ProxyConfig `alloy:",squash"`
	FollowRedirects bool                `alloy:"follow_redirects,attr,optional"`
	Host            string              `alloy:"host,attr,optional"`
	EnableHTTP2     bool                `alloy:"enable_http2,attr,optional"`
	TLSConfig       config.TLSConfig    `alloy:"tls_config,block,optional"`
}

type OAuth struct {
	ClientID     string            `alloy:"client_id,attr"`
	TenantID     string            `alloy:"tenant_id,attr"`
	ClientSecret alloytypes.Secret `alloy:"client_secret,attr"`
}

type ManagedIdentity struct {
	ClientID string `alloy:"client_id,attr"`
}

var DefaultArguments = Arguments{
	Environment:     azure.PublicCloud.Name,
	Port:            80,
	RefreshInterval: 5 * time.Minute,
	FollowRedirects: true,
	EnableHTTP2:     true,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
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
	httpClientConfig.Host = a.Host

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
