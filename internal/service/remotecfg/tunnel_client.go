package remotecfg

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/tunnel/v1/tunnelv1connect"
	commonconfig "github.com/prometheus/common/config"
)

func newTunnelClient(args Arguments) (tunnelv1connect.TunnelServiceClient, error) {
	tunnelURL := args.getTunnelURL()
	endpoint, err := url.Parse(tunnelURL)
	if err != nil {
		return nil, fmt.Errorf("parse Fleet Management URL: %w", err)
	}

	var httpClient *http.Client
	switch strings.ToLower(endpoint.Scheme) {
	case "https":
		httpClient, err = newRemoteConfigHTTPClient(args)
	case "http":
		httpClient, err = newH2CRemoteConfigHTTPClient(args)
	default:
		return nil, fmt.Errorf("unsupported Fleet Management URL scheme %q", endpoint.Scheme)
	}
	if err != nil {
		return nil, err
	}

	return tunnelv1connect.NewTunnelServiceClient(
		httpClient,
		tunnelURL,
		connect.WithInterceptors(newAgentInterceptor()),
	), nil
}

func newH2CRemoteConfigHTTPClient(args Arguments) (*http.Client, error) {
	cfg := args.HTTPClientConfig.Convert()
	if cfg.OAuth2 != nil {
		return nil, fmt.Errorf("OAuth2 authentication is not supported for unencrypted HTTP/2")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = cfg.Proxy()
	transport.ProxyConnectHeader = cfg.GetProxyConnectHeader()
	transport.DisableCompression = true
	transport.Protocols = new(http.Protocols)
	transport.Protocols.SetUnencryptedHTTP2(true)

	roundTripper, err := addH2CAuthorization(cfg, transport)
	if err != nil {
		return nil, err
	}
	roundTripper, err = addH2CBearerToken(cfg, roundTripper)
	if err != nil {
		return nil, err
	}
	roundTripper, err = addH2CBasicAuth(cfg, roundTripper)
	if err != nil {
		return nil, err
	}
	if cfg.HTTPHeaders != nil {
		roundTripper = commonconfig.NewHeadersRoundTripper(cfg.HTTPHeaders, roundTripper)
	}

	client := &http.Client{Transport: roundTripper}
	if !cfg.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client, nil
}

func addH2CAuthorization(cfg *commonconfig.HTTPClientConfig, next http.RoundTripper) (http.RoundTripper, error) {
	if cfg.Authorization != nil {
		credentials, err := newSecretReader(cfg.Authorization.Credentials, cfg.Authorization.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("configure authorization credentials: %w", err)
		}
		authType := cfg.Authorization.Type
		if authType == "" {
			authType = "Bearer"
		}
		return commonconfig.NewAuthorizationCredentialsRoundTripper(authType, credentials, next), nil
	}
	return next, nil
}

func addH2CBearerToken(cfg *commonconfig.HTTPClientConfig, next http.RoundTripper) (http.RoundTripper, error) {
	if cfg.BearerToken != "" || cfg.BearerTokenFile != "" {
		credentials, err := newSecretReader(cfg.BearerToken, cfg.BearerTokenFile)
		if err != nil {
			return nil, fmt.Errorf("configure bearer token: %w", err)
		}
		return commonconfig.NewAuthorizationCredentialsRoundTripper("Bearer", credentials, next), nil
	}
	return next, nil
}

func addH2CBasicAuth(cfg *commonconfig.HTTPClientConfig, next http.RoundTripper) (http.RoundTripper, error) {
	if cfg.BasicAuth != nil {
		username, err := newSecretReader(commonconfig.Secret(cfg.BasicAuth.Username), cfg.BasicAuth.UsernameFile)
		if err != nil {
			return nil, fmt.Errorf("configure basic auth username: %w", err)
		}
		password, err := newSecretReader(cfg.BasicAuth.Password, cfg.BasicAuth.PasswordFile)
		if err != nil {
			return nil, fmt.Errorf("configure basic auth password: %w", err)
		}
		return commonconfig.NewBasicAuthRoundTripper(username, password, next), nil
	}
	return next, nil
}

func newSecretReader(inline commonconfig.Secret, file string) (commonconfig.SecretReader, error) {
	if inline != "" && file != "" {
		return nil, fmt.Errorf("inline and file secret sources are both configured")
	}
	if inline != "" {
		return commonconfig.NewInlineSecret(string(inline)), nil
	}
	if file != "" {
		return commonconfig.NewFileSecret(file), nil
	}
	return nil, nil
}
