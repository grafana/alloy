package config

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientConfigBearerToken(t *testing.T) {
	var exampleRiverConfig = `
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
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigBearerTokenFile(t *testing.T) {
	var exampleRiverConfig = `
	bearer_token_file = "/path/to/file.token"
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true
`

	var httpClientConfig HTTPClientConfig
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigBasicAuthPassword(t *testing.T) {
	var exampleRiverConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	basic_auth {
		username = "user"
		password = "password"
	}
`

	var httpClientConfig HTTPClientConfig
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigBasicAuthPasswordFile(t *testing.T) {
	var exampleRiverConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	basic_auth {
		username = "user"
		password_file = "/path/to/file.password"
	}
`

	var httpClientConfig HTTPClientConfig
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigAuthorizationCredentials(t *testing.T) {
	var exampleRiverConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	authorization {
		type = "Bearer"
		credentials = "credential"
	}
`

	var httpClientConfig HTTPClientConfig
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigAuthorizationCredentialsFile(t *testing.T) {
	var exampleRiverConfig = `
	proxy_url = "http://0.0.0.0:11111"
	follow_redirects = true
	enable_http2 = true

	authorization {
		type = "Bearer"
		credentials_file = "/path/to/file.credentials"
	}
`

	var httpClientConfig HTTPClientConfig
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigOath2ClientSecret(t *testing.T) {
	var exampleRiverConfig = `
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
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestHTTPClientConfigOath2ClientSecretFile(t *testing.T) {
	var exampleRiverConfig = `
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
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
}

func TestOath2TLSConvert(t *testing.T) {
	var exampleRiverConfig = `
	oauth2 {
		client_id = "client_id"
		client_secret_file = "/path/to/file.oath2"
		scopes = ["scope1", "scope2"]
		token_url = "token_url"
		endpoint_params = {"param1" = "value1", "param2" = "value2"}
	}
`

	var httpClientConfig HTTPClientConfig
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.NoError(t, err)
	newCfg := httpClientConfig.Convert()
	require.NotNil(t, newCfg)
}

func TestHTTPClientBadConfig(t *testing.T) {
	var exampleRiverConfig = `
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
	err := river.Unmarshal([]byte(exampleRiverConfig), &httpClientConfig)
	require.ErrorContains(t, err, "at most one of basic_auth password & password_file must be configured")
}
