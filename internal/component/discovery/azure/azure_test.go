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

func TestUnmarshal(t *testing.T) {
	proxyURL, _ := url.Parse("http://example:8080")

	tests := []struct {
		name      string
		config    string
		expectErr string
		expected  Arguments
	}{
		{
			name: "oauth full config",
			config: `
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
				}`,
			expected: Arguments{
				Environment:    "AzureTestCloud",
				Port:           8080,
				SubscriptionID: "subid",
				OAuth: &OAuth{
					ClientID:     "clientid",
					TenantID:     "tenantid",
					ClientSecret: "clientsecret",
				},
				RefreshInterval: 10 * time.Minute,
				ResourceGroup:   "test",
				ProxyConfig:     &config.ProxyConfig{ProxyURL: config.URL{URL: proxyURL}},
				FollowRedirects: false,
				EnableHTTP2:     true,
				HTTPHeaders: &config.Headers{
					Headers: map[string][]alloytypes.Secret{"foo": {"foobar"}},
				},
			},
		},
		{
			name: "managed_identity",
			config: `
				environment = "AzureTestCloud"
				port = 8080
				subscription_id = "subid"
				refresh_interval = "10m"
				resource_group = "test"
				managed_identity {
					client_id = "clientid"
				}`,
			expected: Arguments{
				Environment:     "AzureTestCloud",
				Port:            8080,
				SubscriptionID:  "subid",
				ManagedIdentity: &ManagedIdentity{ClientID: "clientid"},
				RefreshInterval: 10 * time.Minute,
				ResourceGroup:   "test",
				FollowRedirects: true,
				EnableHTTP2:     true,
			},
		},
		{
			name: "sdk_auth with tenant_id",
			config: `
				environment = "AzureTestCloud"
				port = 8080
				subscription_id = "subid"
				refresh_interval = "10m"
				resource_group = "test"
				sdk_auth {
					tenant_id = "tenantid"
				}`,
			expected: Arguments{
				Environment:     "AzureTestCloud",
				Port:            8080,
				SubscriptionID:  "subid",
				SDK:             &SDK{TenantID: "tenantid"},
				RefreshInterval: 10 * time.Minute,
				ResourceGroup:   "test",
				FollowRedirects: true,
				EnableHTTP2:     true,
			},
		},
		{
			name: "sdk_auth without tenant_id",
			config: `
				environment = "AzureTestCloud"
				port = 8080
				subscription_id = "subid"
				refresh_interval = "10m"
				resource_group = "test"
				sdk_auth {}`,
			expected: Arguments{
				Environment:     "AzureTestCloud",
				Port:            8080,
				SubscriptionID:  "subid",
				SDK:             &SDK{},
				RefreshInterval: 10 * time.Minute,
				ResourceGroup:   "test",
				FollowRedirects: true,
				EnableHTTP2:     true,
			},
		},
		{
			name: "workload_identity",
			config: `
				environment = "AzureTestCloud"
				port = 8080
				subscription_id = "subid"
				refresh_interval = "10m"
				resource_group = "test"
				workload_identity {}`,
			expected: Arguments{
				Environment:      "AzureTestCloud",
				Port:             8080,
				SubscriptionID:   "subid",
				WorkloadIdentity: &WorkloadIdentity{},
				RefreshInterval:  10 * time.Minute,
				ResourceGroup:    "test",
				FollowRedirects:  true,
				EnableHTTP2:      true,
			},
		},
		{
			name: "oauth missing required fields",
			config: `
				environment = "AzureTestCloud"
				port = 8080
				subscription_id = "subid"
				refresh_interval = "10m"
				resource_group = "test"
				oauth {
					client_id = "clientid"
				}`,
			expectErr: "missing required attribute",
		},
		{
			name: "no auth method",
			config: `
				environment = "AzureTestCloud"
				port = 8080
				subscription_id = "subid"
				refresh_interval = "10m"
				resource_group = "test"`,
			expectErr: "exactly one of oauth, managed_identity, sdk_auth, or workload_identity must be specified",
		},
		{
			name: "multiple auth methods",
			config: `
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
				}`,
			expectErr: "exactly one of oauth, managed_identity, sdk_auth, or workload_identity must be specified",
		},
		{
			name: "invalid tls config",
			config: `
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
				}`,
			expectErr: "at most one of cert_pem and cert_file must be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.config), &args)
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, args)
		})
	}
}

func TestConvert(t *testing.T) {
	proxyURL, _ := url.Parse("http://example:8080")
	proxyConfig := &config.ProxyConfig{ProxyURL: config.URL{URL: proxyURL}}

	tests := []struct {
		name string
		args Arguments
		// expected is compared against the converted SDConfig. HTTPClientConfig
		// is derived from Prometheus defaults, so it's verified separately below
		// via wantProxyURL/wantHeader rather than reconstructed here.
		expected     promdiscovery.SDConfig
		wantProxyURL string
		wantHeader   string
	}{
		{
			name: "oauth",
			args: Arguments{
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
				ProxyConfig:     proxyConfig,
				HTTPHeaders: &config.Headers{
					Headers: map[string][]alloytypes.Secret{"foo": {"foobar"}},
				},
			},
			expected: promdiscovery.SDConfig{
				Environment:          "AzureTestCloud",
				Port:                 8080,
				SubscriptionID:       "subid",
				RefreshInterval:      model.Duration(10 * time.Minute),
				ResourceGroup:        "test",
				AuthenticationMethod: "OAuth",
				ClientID:             "clientid",
				TenantID:             "tenantid",
				ClientSecret:         "clientsecret",
			},
			wantProxyURL: "http://example:8080",
			wantHeader:   "foobar",
		},
		{
			name: "managed_identity",
			args: Arguments{
				Environment:     "AzureTestCloud",
				Port:            8080,
				SubscriptionID:  "subid",
				RefreshInterval: 10 * time.Minute,
				ResourceGroup:   "test",
				ManagedIdentity: &ManagedIdentity{ClientID: "clientid"},
				FollowRedirects: true,
				EnableHTTP2:     true,
				ProxyConfig:     proxyConfig,
			},
			expected: promdiscovery.SDConfig{
				Environment:          "AzureTestCloud",
				Port:                 8080,
				SubscriptionID:       "subid",
				RefreshInterval:      model.Duration(10 * time.Minute),
				ResourceGroup:        "test",
				AuthenticationMethod: "ManagedIdentity",
				ClientID:             "clientid",
			},
			wantProxyURL: "http://example:8080",
		},
		{
			name: "sdk_auth",
			args: Arguments{
				Environment:     "AzureTestCloud",
				Port:            8080,
				SubscriptionID:  "subid",
				RefreshInterval: 10 * time.Minute,
				ResourceGroup:   "test",
				SDK:             &SDK{TenantID: "tenantid"},
			},
			expected: promdiscovery.SDConfig{
				Environment:          "AzureTestCloud",
				Port:                 8080,
				SubscriptionID:       "subid",
				RefreshInterval:      model.Duration(10 * time.Minute),
				ResourceGroup:        "test",
				AuthenticationMethod: "SDK",
				TenantID:             "tenantid",
			},
		},
		{
			name: "workload_identity",
			args: Arguments{
				Environment:      "AzureTestCloud",
				Port:             8080,
				SubscriptionID:   "subid",
				RefreshInterval:  10 * time.Minute,
				ResourceGroup:    "test",
				WorkloadIdentity: &WorkloadIdentity{},
			},
			expected: promdiscovery.SDConfig{
				Environment:          "AzureTestCloud",
				Port:                 8080,
				SubscriptionID:       "subid",
				RefreshInterval:      model.Duration(10 * time.Minute),
				ResourceGroup:        "test",
				AuthenticationMethod: "WorkloadIdentity",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.args.Convert().(*promdiscovery.SDConfig)
			require.True(t, ok)

			// The HTTP client config is derived from Prometheus defaults; verify
			// its relevant fields separately and exclude it from the struct diff.
			httpCfg := got.HTTPClientConfig
			got.HTTPClientConfig = tc.expected.HTTPClientConfig
			require.Equal(t, &tc.expected, got)

			assert.Equal(t, tc.args.FollowRedirects, httpCfg.FollowRedirects)
			assert.Equal(t, tc.args.EnableHTTP2, httpCfg.EnableHTTP2)
			if tc.wantProxyURL != "" {
				assert.Equal(t, tc.wantProxyURL, httpCfg.ProxyURL.String())
			}
			if tc.wantHeader != "" {
				assert.Equal(t, tc.wantHeader, string(httpCfg.HTTPHeaders.Headers["foo"].Secrets[0]))
			}
		})
	}
}
