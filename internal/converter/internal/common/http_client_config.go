package common

import (
	"reflect"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/syntax/alloytypes"
	prom_config "github.com/prometheus/common/config"
)

func ToHttpClientConfig(httpClientConfig *prom_config.HTTPClientConfig) *config.HTTPClientConfig {
	if httpClientConfig == nil {
		return nil
	}

	return &config.HTTPClientConfig{
		BasicAuth:       toBasicAuth(httpClientConfig.BasicAuth),
		Authorization:   toAuthorization(httpClientConfig.Authorization),
		OAuth2:          toOAuth2(httpClientConfig.OAuth2),
		BearerToken:     alloytypes.Secret(httpClientConfig.BearerToken),
		BearerTokenFile: httpClientConfig.BearerTokenFile,
		ProxyConfig:     ToProxyConfig(httpClientConfig.ProxyConfig),
		TLSConfig:       *ToTLSConfig(&httpClientConfig.TLSConfig),
		FollowRedirects: httpClientConfig.FollowRedirects,
		EnableHTTP2:     httpClientConfig.EnableHTTP2,
	}
}

// ValidateHttpClientConfig returns [diag.Diagnostics] for currently
// unsupported Alloy features available in Prometheus.
func ValidateHttpClientConfig(httpClientConfig *prom_config.HTTPClientConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddAll(ValidateSupported(NotEquals, httpClientConfig.TLSConfig.MaxVersion, prom_config.TLSVersion(0), "HTTP Client max_version", ""))

	return diags
}

func toBasicAuth(basicAuth *prom_config.BasicAuth) *config.BasicAuth {
	if basicAuth == nil {
		return nil
	}

	return &config.BasicAuth{
		Username:     basicAuth.Username,
		Password:     alloytypes.Secret(basicAuth.Password),
		PasswordFile: basicAuth.PasswordFile,
	}
}

func toAuthorization(authorization *prom_config.Authorization) *config.Authorization {
	if authorization == nil {
		return nil
	}

	return &config.Authorization{
		Type:            authorization.Type,
		Credentials:     alloytypes.Secret(authorization.Credentials),
		CredentialsFile: authorization.CredentialsFile,
	}
}

func toOAuth2(oAuth2 *prom_config.OAuth2) *config.OAuth2Config {
	if oAuth2 == nil {
		return nil
	}

	return &config.OAuth2Config{
		ClientID:         oAuth2.ClientID,
		ClientSecret:     alloytypes.Secret(oAuth2.ClientSecret),
		ClientSecretFile: oAuth2.ClientSecretFile,
		Scopes:           oAuth2.Scopes,
		TokenURL:         oAuth2.TokenURL,
		EndpointParams:   oAuth2.EndpointParams,
		ProxyConfig:      ToProxyConfig(oAuth2.ProxyConfig),
		TLSConfig:        ToTLSConfig(&oAuth2.TLSConfig),
	}
}

func ToProxyConfig(proxyConfig prom_config.ProxyConfig) *config.ProxyConfig {
	// Prometheus proxy config is not a pointer so treat the default struct as nil
	if reflect.DeepEqual(proxyConfig, prom_config.ProxyConfig{}) {
		return nil
	}

	return &config.ProxyConfig{
		ProxyURL:             toProxyURL(proxyConfig.ProxyURL),
		NoProxy:              proxyConfig.NoProxy,
		ProxyFromEnvironment: proxyConfig.ProxyFromEnvironment,
		ProxyConnectHeader:   toProxyConnectHeader(proxyConfig.ProxyConnectHeader),
	}
}

func toProxyURL(proxyURL prom_config.URL) config.URL {
	if proxyURL.URL == nil {
		return config.URL{}
	}

	return config.URL{
		URL: proxyURL.URL,
	}
}

func toProxyConnectHeader(proxyConnectHeader prom_config.ProxyHeader) config.ProxyHeader {
	if proxyConnectHeader == nil {
		return config.ProxyHeader{}
	}

	header := config.ProxyHeader{
		Header: make(map[string][]alloytypes.Secret),
	}
	for name, values := range proxyConnectHeader {
		var s []alloytypes.Secret
		if values != nil {
			s = make([]alloytypes.Secret, 0, len(values))
			for _, value := range values {
				s = append(s, alloytypes.Secret(value))
			}
		}
		header.Header[name] = s
	}
	return header
}

func ToTLSConfig(tlsConfig *prom_config.TLSConfig) *config.TLSConfig {
	if tlsConfig == nil {
		return nil
	}

	return &config.TLSConfig{
		CA:                 tlsConfig.CA,
		CAFile:             tlsConfig.CAFile,
		Cert:               tlsConfig.Cert,
		CertFile:           tlsConfig.CertFile,
		Key:                alloytypes.Secret(tlsConfig.Key),
		KeyFile:            tlsConfig.KeyFile,
		ServerName:         tlsConfig.ServerName,
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
		MinVersion:         config.TLSVersion(tlsConfig.MinVersion),
	}
}
