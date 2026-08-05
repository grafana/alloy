package config

import (
	"crypto/tls"
	"net/url"
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientConfigBearerToken(t *testing.T) {
	var exampleAlloyConfig = `
	bearer_token = "token"
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	tls_config {
		ca_file = "/path/to/file.ca"
		cert_file = "/path/to/file.cert"
		key_file = "/path/to/file.key"
		server_name = "server_name"
		insecure_skip_verify = false
		min_version = "TLS13"
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigBearerTokenFile(t *testing.T) {
	var exampleAlloyConfig = `
	bearer_token_file = "/path/to/file.token"
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigBasicAuthPassword(t *testing.T) {
	var exampleAlloyConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	basic_auth {
		username = "user"
		password = "password"
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigBasicAuthPasswordFile(t *testing.T) {
	var exampleAlloyConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	basic_auth {
		username = "user"
		password_file = "/path/to/file.password"
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigAuthorizationCredentials(t *testing.T) {
	var exampleAlloyConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	authorization {
		type = "Bearer"
		credentials = "credential"
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigAuthorizationCredentialsFile(t *testing.T) {
	var exampleAlloyConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	authorization {
		type = "Bearer"
		credentials_file = "/path/to/file.credentials"
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigOath2ClientSecret(t *testing.T) {
	var exampleAlloyConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	oauth2 {
		client_id = "client_id"
		client_secret = "client_secret"
		scopes = ["scope1", "scope2"]
		token_url = "token_url"
		endpoint_params = {"param1" = "value1", "param2" = "value2"}
		proxy_url = "http://0.0.0.0:11111"
		tls_config {
			ca_file = "/path/to/file.ca"
			cert_file = "/path/to/file.cert"
			key_file = "/path/to/file.key"
			server_name = "server_name"
			insecure_skip_verify = false
			min_version = "TLS13"
		}
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigOath2ClientSecretFile(t *testing.T) {
	var exampleAlloyConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	oauth2 {
		client_id = "client_id"
		client_secret_file = "/path/to/file.oath2"
		scopes = ["scope1", "scope2"]
		token_url = "token_url"
		endpoint_params = {"param1" = "value1", "param2" = "value2"}
		proxy_url = "http://0.0.0.0:11111"
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestOath2TLSConvert(t *testing.T) {
	var exampleAlloyConfig = `
	oauth2 {
		client_id = "client_id"
		client_secret_file = "/path/to/file.oath2"
		scopes = ["scope1", "scope2"]
		token_url = "token_url"
		endpoint_params = {"param1" = "value1", "param2" = "value2"}
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.NoError(t, err)
	newCfg := httpClientConfig.Convert()
	require.NotNil(t, newCfg)
}

func TestHTTPClientBadConfig(t *testing.T) {
	var exampleAlloyConfig = `
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	basic_auth {
		username = "user"
		password = "password"
		password_file = "/path/to/file.password"
	}

	authorization {
		type = "Bearer"
		credentials = "credential"
		credentials_file = "/path/to/file.credentials"
	}

	oauth2 {
		client_id = "client_id"
		client_secret = "client_secret"
		client_secret_file = "/path/to/file.oath2"
		scopes = ["scope1", "scope2"]
		token_url = "token_url"
		endpoint_params = {"param1" = "value1", "param2" = "value2"}
		proxy_url = "http://0.0.0.0:11111"
		tls_config {
			ca_file = "/path/to/file.ca"
			cert_file = "/path/to/file.cert"
			key_file = "/path/to/file.key"
			server_name = "server_name"
			insecure_skip_verify = false
			min_version = "TLS13"
		}
	}

	tls_config {
		ca_file = "/path/to/file.ca"
		cert_file = "/path/to/file.cert"
		key_file = "/path/to/file.key"
		server_name = "server_name"
		insecure_skip_verify = false
		min_version = "TLS13"
	}
`

	var httpClientConfig HTTPClientConfig
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &httpClientConfig)
	require.ErrorContains(t, err, "at most one of basic_auth password & password_file must be configured")
}

func TestHTTPClientConfigEqual(t *testing.T) {
	tests := []struct {
		desc     string
		a, b     *HTTPClientConfig
		expected bool
	}{
		{
			desc:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			desc:     "nil and default",
			a:        nil,
			b:        CloneDefaultHTTPClientConfig(),
			expected: false,
		},
		{
			desc:     "two defaults",
			a:        CloneDefaultHTTPClientConfig(),
			b:        CloneDefaultHTTPClientConfig(),
			expected: true,
		},
		{
			desc: "all fields set and identical",
			a: &HTTPClientConfig{
				BasicAuth: &BasicAuth{Username: "user", Password: "password"},
				ProxyConfig: &ProxyConfig{
					ProxyURL:           mustParseURL(t, "http://0.0.0.0:11111"),
					NoProxy:            "localhost",
					ProxyConnectHeader: ProxyHeader{Header: map[string][]alloytypes.Secret{"X-Proxy": {"a", "b"}}},
				},
				TLSConfig: TLSConfig{
					CAFile:     "/path/to/file.ca",
					ServerName: "server_name",
					MinVersion: TLSVersion(tls.VersionTLS13),
				},
				FollowRedirects: true,
				EnableHTTP2:     true,
				HTTPHeaders:     &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"value"}}},
			},
			b: &HTTPClientConfig{
				BasicAuth: &BasicAuth{Username: "user", Password: "password"},
				ProxyConfig: &ProxyConfig{
					ProxyURL:           mustParseURL(t, "http://0.0.0.0:11111"),
					NoProxy:            "localhost",
					ProxyConnectHeader: ProxyHeader{Header: map[string][]alloytypes.Secret{"X-Proxy": {"a", "b"}}},
				},
				TLSConfig: TLSConfig{
					CAFile:     "/path/to/file.ca",
					ServerName: "server_name",
					MinVersion: TLSVersion(tls.VersionTLS13),
				},
				FollowRedirects: true,
				EnableHTTP2:     true,
				HTTPHeaders:     &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"value"}}},
			},
			expected: true,
		},
		{
			desc:     "different bearer token",
			a:        &HTTPClientConfig{BearerToken: "token"},
			b:        &HTTPClientConfig{BearerToken: "other-token"},
			expected: false,
		},
		{
			desc:     "different bearer token file",
			a:        &HTTPClientConfig{BearerTokenFile: "/path/to/file.token"},
			b:        &HTTPClientConfig{BearerTokenFile: "/other/path/to/file.token"},
			expected: false,
		},
		{
			desc:     "unset and set basic auth",
			a:        &HTTPClientConfig{},
			b:        &HTTPClientConfig{BasicAuth: &BasicAuth{Username: "user"}},
			expected: false,
		},
		{
			desc:     "different basic auth password",
			a:        &HTTPClientConfig{BasicAuth: &BasicAuth{Username: "user", Password: "password"}},
			b:        &HTTPClientConfig{BasicAuth: &BasicAuth{Username: "user", Password: "other-password"}},
			expected: false,
		},
		{
			desc:     "different authorization credentials",
			a:        &HTTPClientConfig{Authorization: &Authorization{Type: "Bearer", Credentials: "credential"}},
			b:        &HTTPClientConfig{Authorization: &Authorization{Type: "Bearer", Credentials: "other-credential"}},
			expected: false,
		},
		{
			desc:     "different oauth2 client id",
			a:        &HTTPClientConfig{OAuth2: &OAuth2Config{ClientID: "client_id", TokenURL: "token_url"}},
			b:        &HTTPClientConfig{OAuth2: &OAuth2Config{ClientID: "other_client_id", TokenURL: "token_url"}},
			expected: false,
		},
		{
			desc:     "reordered oauth2 scopes",
			a:        &HTTPClientConfig{OAuth2: &OAuth2Config{Scopes: []string{"scope1", "scope2"}}},
			b:        &HTTPClientConfig{OAuth2: &OAuth2Config{Scopes: []string{"scope2", "scope1"}}},
			expected: false,
		},
		{
			desc:     "different oauth2 endpoint params",
			a:        &HTTPClientConfig{OAuth2: &OAuth2Config{EndpointParams: map[string]string{"param1": "value1"}}},
			b:        &HTTPClientConfig{OAuth2: &OAuth2Config{EndpointParams: map[string]string{"param1": "value2"}}},
			expected: false,
		},
		{
			desc:     "different oauth2 tls config",
			a:        &HTTPClientConfig{OAuth2: &OAuth2Config{TLSConfig: &TLSConfig{CAFile: "/path/to/file.ca"}}},
			b:        &HTTPClientConfig{OAuth2: &OAuth2Config{TLSConfig: &TLSConfig{CAFile: "/other/path/to/file.ca"}}},
			expected: false,
		},
		{
			desc:     "different tls min version",
			a:        &HTTPClientConfig{TLSConfig: TLSConfig{MinVersion: TLSVersion(tls.VersionTLS12)}},
			b:        &HTTPClientConfig{TLSConfig: TLSConfig{MinVersion: TLSVersion(tls.VersionTLS13)}},
			expected: false,
		},
		{
			desc:     "different tls ca file",
			a:        &HTTPClientConfig{TLSConfig: TLSConfig{CAFile: "/path/to/file.ca"}},
			b:        &HTTPClientConfig{TLSConfig: TLSConfig{CAFile: "/other/path/to/file.ca"}},
			expected: false,
		},
		{
			desc:     "different follow redirects",
			a:        &HTTPClientConfig{FollowRedirects: true},
			b:        &HTTPClientConfig{FollowRedirects: false},
			expected: false,
		},
		{
			desc:     "different enable http2",
			a:        &HTTPClientConfig{EnableHTTP2: true},
			b:        &HTTPClientConfig{EnableHTTP2: false},
			expected: false,
		},
		{
			desc:     "unset and set proxy config",
			a:        &HTTPClientConfig{},
			b:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{}},
			expected: false,
		},
		{
			desc:     "same proxy url parsed twice",
			a:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:11111")}},
			b:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:11111")}},
			expected: true,
		},
		{
			desc:     "different proxy url",
			a:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:11111")}},
			b:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:22222")}},
			expected: false,
		},
		{
			desc:     "unset and set proxy url",
			a:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{}},
			b:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{ProxyURL: mustParseURL(t, "http://0.0.0.0:11111")}},
			expected: false,
		},
		{
			desc:     "different proxy connect header",
			a:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{ProxyConnectHeader: ProxyHeader{Header: map[string][]alloytypes.Secret{"X-Proxy": {"a"}}}}},
			b:        &HTTPClientConfig{ProxyConfig: &ProxyConfig{ProxyConnectHeader: ProxyHeader{Header: map[string][]alloytypes.Secret{"X-Proxy": {"b"}}}}},
			expected: false,
		},
		{
			desc:     "unset and empty http headers",
			a:        &HTTPClientConfig{},
			b:        &HTTPClientConfig{HTTPHeaders: &Headers{}},
			expected: false,
		},
		{
			desc:     "nil and empty http header map",
			a:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: nil}},
			b:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: map[string][]alloytypes.Secret{}}},
			expected: true,
		},
		{
			desc:     "different http header value",
			a:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"value"}}}},
			b:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"other-value"}}}},
			expected: false,
		},
		{
			desc:     "extra http header",
			a:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"value"}}}},
			b:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"value"}, "X-Other": {"value"}}}},
			expected: false,
		},
		{
			desc:     "reordered http header values",
			a:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"a", "b"}}}},
			b:        &HTTPClientConfig{HTTPHeaders: &Headers{Headers: map[string][]alloytypes.Secret{"X-Test": {"b", "a"}}}},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.a.Equal(tc.b))
			require.Equal(t, tc.expected, tc.b.Equal(tc.a))
		})
	}
}

func mustParseURL(t *testing.T, s string) URL {
	t.Helper()

	u, err := url.Parse(s)
	require.NoError(t, err)
	return URL{URL: u}
}
