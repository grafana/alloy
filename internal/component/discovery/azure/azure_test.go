package azure

import (
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	promdiscovery "github.com/prometheus/prometheus/discovery/azure"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyCfg := `
		environment = "AzureTestCloud"
		port = 8080
		subscription_id = "subid"
		refresh_interval = "10m"
		resource_group = "test"
		oauth {
			client_id = "clientid"
			tenant_id = "tenantid"
			client_secret = "clientsecret"
		}
		enable_http2 = true
		follow_redirects = false
		proxy_url = "http://example:8080"
		http_headers = {
			"foo" = ["foobar"],
		}`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	assert.Equal(t, "AzureTestCloud", args.Environment)
	assert.Equal(t, 8080, args.Port)
	assert.Equal(t, "subid", args.SubscriptionID)
	assert.Equal(t, 10*time.Minute, args.RefreshInterval)
	assert.Equal(t, "test", args.ResourceGroup)
	assert.Equal(t, "clientid", args.OAuth.ClientID)
	assert.Equal(t, "tenantid", args.OAuth.TenantID)
	assert.Equal(t, "clientsecret", string(args.OAuth.ClientSecret))
	assert.Equal(t, true, args.EnableHTTP2)
	assert.Equal(t, false, args.FollowRedirects)
	assert.Equal(t, "http://example:8080", args.ProxyConfig.ProxyURL.String())

	header := args.HTTPHeaders.Headers["foo"][0]
	assert.Equal(t, "foobar", string(header))
}

func TestAlloyUnmarshal_OAuthRequiredFields(t *testing.T) {
	alloyCfg := `
		environment = "AzureTestCloud"
		port = 8080
		subscription_id = "subid"
		refresh_interval = "10m"
		resource_group = "test"
		oauth {
			client_id = "clientid"
		}`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.Error(t, err)
}

func TestValidate(t *testing.T) {
	noAuth := `
		environment = "AzureTestCloud"
		port = 8080
		subscription_id = "subid"
		refresh_interval = "10m"
		resource_group = "test"`

	var args Arguments
	err := syntax.Unmarshal([]byte(noAuth), &args)
	require.ErrorContains(t, err, "exactly one of oauth or managed_identity must be specified")

	bothAuth := `
		environment = "AzureTestCloud"
		port = 8080
		subscription_id = "subid"
		refresh_interval = "10m"
		resource_group = "test"
		oauth {
			client_id = "clientid"
			tenant_id = "tenantid"
			client_secret = "clientsecret"
		}
		managed_identity {
			client_id = "clientid"
		}`
	var args2 Arguments
	err = syntax.Unmarshal([]byte(bothAuth), &args2)
	require.ErrorContains(t, err, "exactly one of oauth or managed_identity must be specified")

	invalidTLS := `
		environment = "AzureTestCloud"
		port = 8080
		subscription_id = "subid"
		refresh_interval = "10m"
		resource_group = "test"
		managed_identity {
			client_id = "clientid"
		}
		tls_config {
			cert_file = "certfile"
			cert_pem = "certpem"
		}`
	var args3 Arguments
	err = syntax.Unmarshal([]byte(invalidTLS), &args3)
	require.ErrorContains(t, err, "at most one of cert_pem and cert_file must be configured")
}

func TestConvert(t *testing.T) {
	proxyUrl, _ := url.Parse("http://example:8080")
	alloyArgsOAuth := Arguments{
		Environment:     "AzureTestCloud",
		Port:            8080,
		SubscriptionID:  "subid",
		RefreshInterval: 10 * time.Minute,
		ResourceGroup:   "test",
		OAuth: &OAuth{
			ClientID:     "clientid",
			TenantID:     "tenantid",
			ClientSecret: "clientsecret",
		},
		FollowRedirects: false,
		EnableHTTP2:     false,
		ProxyConfig: &config.ProxyConfig{
			ProxyURL: config.URL{
				URL: proxyUrl,
			},
		},
		HTTPHeaders: &config.Headers{
			Headers: map[string][]alloytypes.Secret{
				"foo": {"foobar"},
			},
		},
	}

	args := alloyArgsOAuth.Convert()
	promArgs, ok := args.(*promdiscovery.SDConfig)
	require.True(t, ok)
	assert.Equal(t, "AzureTestCloud", promArgs.Environment)
	assert.Equal(t, 8080, promArgs.Port)
	assert.Equal(t, "subid", promArgs.SubscriptionID)
	assert.Equal(t, model.Duration(10*time.Minute), promArgs.RefreshInterval)
	assert.Equal(t, "test", promArgs.ResourceGroup)
	assert.Equal(t, "clientid", promArgs.ClientID)
	assert.Equal(t, "tenantid", promArgs.TenantID)
	assert.Equal(t, "clientsecret", string(promArgs.ClientSecret))
	assert.Equal(t, false, promArgs.HTTPClientConfig.FollowRedirects)
	assert.Equal(t, false, promArgs.HTTPClientConfig.EnableHTTP2)
	assert.Equal(t, "http://example:8080", promArgs.HTTPClientConfig.ProxyURL.String())

	header := promArgs.HTTPClientConfig.HTTPHeaders.Headers["foo"].Secrets[0]
	assert.Equal(t, "foobar", string(header))

	alloyArgsManagedIdentity := Arguments{
		Environment:     "AzureTestCloud",
		Port:            8080,
		SubscriptionID:  "subid",
		RefreshInterval: 10 * time.Minute,
		ResourceGroup:   "test",
		ManagedIdentity: &ManagedIdentity{
			ClientID: "clientid",
		},
		FollowRedirects: true,
		EnableHTTP2:     true,
		ProxyConfig: &config.ProxyConfig{
			ProxyURL: config.URL{
				URL: proxyUrl,
			},
		},
	}

	args = alloyArgsManagedIdentity.Convert()
	promArgs, ok = args.(*promdiscovery.SDConfig)
	require.True(t, ok)
	assert.Equal(t, "AzureTestCloud", promArgs.Environment)
	assert.Equal(t, 8080, promArgs.Port)
	assert.Equal(t, "subid", promArgs.SubscriptionID)
	assert.Equal(t, model.Duration(10*time.Minute), promArgs.RefreshInterval)
	assert.Equal(t, "test", promArgs.ResourceGroup)
	assert.Equal(t, "clientid", promArgs.ClientID)
	assert.Equal(t, "", promArgs.TenantID)
	assert.Equal(t, "", string(promArgs.ClientSecret))
	assert.Equal(t, true, promArgs.HTTPClientConfig.FollowRedirects)
	assert.Equal(t, true, promArgs.HTTPClientConfig.EnableHTTP2)
	assert.Equal(t, "http://example:8080", promArgs.HTTPClientConfig.ProxyURL.String())
}
