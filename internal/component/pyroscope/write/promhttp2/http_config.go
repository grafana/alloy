// Copyright 2016 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promhttp2

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	commonconfig "github.com/prometheus/common/config"

	"github.com/mwitkow/go-conntrack"
	"golang.org/x/net/http2"
)

// HTTPClientConfigMirror wraps commonconfig.HTTPClientConfig with
// extra fields that cannot be expressed via the upstream type.
type HTTPClientConfigMirror struct {
	commonconfig.HTTPClientConfig
	H2C bool // use HTTP/2 cleartext (h2c) instead of standard transport
}

type httpClientOptions struct {
	dialContextFunc   commonconfig.DialContextFunc
	newTLSConfigFunc  commonconfig.NewTLSConfigFunc
	keepAlivesEnabled bool
	http2Enabled      bool
	idleConnTimeout   time.Duration
	userAgent         string
	host              string
	secretManager     commonconfig.SecretManager
}

// HTTPClientOption defines an option that can be applied to the HTTP client.
type HTTPClientOption interface {
	applyToHTTPClientOptions(options *httpClientOptions)
}

type httpClientOptionFunc func(options *httpClientOptions)

func (f httpClientOptionFunc) applyToHTTPClientOptions(options *httpClientOptions) {
	f(options)
}

// WithDialContextFunc allows you to override the func gets used for the dialing.
// The default is `net.Dialer.DialContext`.
func WithDialContextFunc(fn commonconfig.DialContextFunc) HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.dialContextFunc = fn
	})
}

// WithNewTLSConfigFunc allows you to override the func that creates the TLS config
// from the prometheus http config.
// The default is `NewTLSConfigWithContext`.
func WithNewTLSConfigFunc(newTLSConfigFunc commonconfig.NewTLSConfigFunc) HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.newTLSConfigFunc = newTLSConfigFunc
	})
}

// WithKeepAlivesDisabled allows to disable HTTP keepalive.
func WithKeepAlivesDisabled() HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.keepAlivesEnabled = false
	})
}

// WithHTTP2Disabled allows to disable HTTP2.
func WithHTTP2Disabled() HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.http2Enabled = false
	})
}

// WithIdleConnTimeout allows setting the idle connection timeout.
func WithIdleConnTimeout(timeout time.Duration) HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.idleConnTimeout = timeout
	})
}

// WithUserAgent allows setting the user agent.
func WithUserAgent(ua string) HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.userAgent = ua
	})
}

// WithHost allows setting the host header.
func WithHost(host string) HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.host = host
	})
}

// WithSecretManager allows setting the secret manager.
func WithSecretManager(sm commonconfig.SecretManager) HTTPClientOption {
	return httpClientOptionFunc(func(opts *httpClientOptions) {
		opts.secretManager = sm
	})
}

const (
	grantTypeJWTBearer = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

var (
	// defaultHTTPClientOptions holds the default HTTP client options.
	defaultHTTPClientOptions = httpClientOptions{
		keepAlivesEnabled: true,
		http2Enabled:      true,
		// 5 minutes is typically above the maximum sane scrape interval. So we can
		// use keepalive for all configurations.
		idleConnTimeout:  5 * time.Minute,
		newTLSConfigFunc: commonconfig.NewTLSConfigWithContext,
	}
)

// NewClient returns a http.Client using the specified http.RoundTripper.
func newClient(rt http.RoundTripper) *http.Client {
	return &http.Client{Transport: rt}
}

func NewClientFromConfig(cfg commonconfig.HTTPClientConfig, name string, optFuncs ...HTTPClientOption) (*http.Client, error) {
	return NewClientFromConfigMirror(HTTPClientConfigMirror{HTTPClientConfig: cfg}, name, optFuncs...)
}

func NewClientFromConfigMirror(cfg HTTPClientConfigMirror, name string, optFuncs ...HTTPClientOption) (*http.Client, error) {
	rt, err := newRoundTripperFromConfigWithContext(context.Background(), cfg, name, optFuncs...)
	if err != nil {
		return nil, err
	}
	client := newClient(rt)
	if !cfg.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client, nil
}

// NewRoundTripperFromConfig returns a new HTTP RoundTripper configured for the
// given config.HTTPClientConfig and config.HTTPClientOption.
// The name is used as go-conntrack metric label.
func NewRoundTripperFromConfig(cfg commonconfig.HTTPClientConfig, name string, optFuncs ...HTTPClientOption) (http.RoundTripper, error) {
	return newRoundTripperFromConfigWithContext(context.Background(), HTTPClientConfigMirror{HTTPClientConfig: cfg}, name, optFuncs...)
}

// newRoundTripperFromConfigWithContext returns a new HTTP RoundTripper configured for the
// given config.HTTPClientConfig and config.HTTPClientOption.
// The name is used as go-conntrack metric label.
func newRoundTripperFromConfigWithContext(ctx context.Context, cfg HTTPClientConfigMirror, name string, optFuncs ...HTTPClientOption) (http.RoundTripper, error) {
	opts := defaultHTTPClientOptions
	for _, opt := range optFuncs {
		opt.applyToHTTPClientOptions(&opts)
	}

	var dialContext func(ctx context.Context, network, addr string) (net.Conn, error)

	if opts.dialContextFunc != nil {
		dialContext = conntrack.NewDialContextFunc(
			conntrack.DialWithDialContextFunc((func(context.Context, string, string) (net.Conn, error))(opts.dialContextFunc)),
			conntrack.DialWithTracing(),
			conntrack.DialWithName(name))
	} else {
		dialContext = conntrack.NewDialContextFunc(
			conntrack.DialWithTracing(),
			conntrack.DialWithName(name))
	}

	newRT := func(tlsConfig *tls.Config) (http.RoundTripper, error) {
		// The only timeout we care about is the configured scrape timeout.
		// It is applied on request. So we leave out any timings here.
		var rt http.RoundTripper
		if cfg.H2C {
			rt = &http2.Transport{
				AllowHTTP: true,
				DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
					return dialContext(ctx, network, addr)
				},
				ReadIdleTimeout: time.Minute,
			}
		} else {
			rt = &http.Transport{
				Proxy:                 cfg.Proxy(),
				ProxyConnectHeader:    cfg.GetProxyConnectHeader(),
				MaxIdleConns:          20000,
				MaxIdleConnsPerHost:   1000, // see https://github.com/golang/go/issues/13801
				DisableKeepAlives:     !opts.keepAlivesEnabled,
				TLSClientConfig:       tlsConfig,
				DisableCompression:    true,
				IdleConnTimeout:       opts.idleConnTimeout,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DialContext:           dialContext,
			}
			if opts.http2Enabled && cfg.EnableHTTP2 {
				http2t, err := http2.ConfigureTransports(rt.(*http.Transport))
				if err != nil {
					return nil, err
				}
				http2t.ReadIdleTimeout = time.Minute
			}
		}

		// If a authorization_credentials is provided, create a round tripper that will set the
		// Authorization header correctly on each request.
		if cfg.Authorization != nil {
			credentialsSecret, err := toSecret(opts.secretManager, cfg.Authorization.Credentials, cfg.Authorization.CredentialsFile, cfg.Authorization.CredentialsRef)
			if err != nil {
				return nil, fmt.Errorf("unable to use credentials: %w", err)
			}
			rt = commonconfig.NewAuthorizationCredentialsRoundTripper(cfg.Authorization.Type, credentialsSecret, rt)
		}
		// Backwards compatibility, be nice with importers who would not have
		// called Validate().
		if len(cfg.BearerToken) > 0 || len(cfg.BearerTokenFile) > 0 {
			bearerSecret, err := toSecret(opts.secretManager, cfg.BearerToken, cfg.BearerTokenFile, "")
			if err != nil {
				return nil, fmt.Errorf("unable to use bearer token: %w", err)
			}
			rt = commonconfig.NewAuthorizationCredentialsRoundTripper("Bearer", bearerSecret, rt)
		}

		if cfg.BasicAuth != nil {
			usernameSecret, err := toSecret(opts.secretManager, commonconfig.Secret(cfg.BasicAuth.Username), cfg.BasicAuth.UsernameFile, cfg.BasicAuth.UsernameRef)
			if err != nil {
				return nil, fmt.Errorf("unable to use username: %w", err)
			}
			passwordSecret, err := toSecret(opts.secretManager, cfg.BasicAuth.Password, cfg.BasicAuth.PasswordFile, cfg.BasicAuth.PasswordRef)
			if err != nil {
				return nil, fmt.Errorf("unable to use password: %w", err)
			}
			rt = commonconfig.NewBasicAuthRoundTripper(usernameSecret, passwordSecret, rt)
		}

		if cfg.OAuth2 != nil {
			var (
				oauthCredential commonconfig.SecretReader
				err             error
			)

			if cfg.OAuth2.GrantType == grantTypeJWTBearer {
				oauthCredential, err = toSecret(opts.secretManager, cfg.OAuth2.ClientCertificateKey, cfg.OAuth2.ClientCertificateKeyFile, cfg.OAuth2.ClientCertificateKeyRef)
				if err != nil {
					return nil, fmt.Errorf("unable to use client certificate: %w", err)
				}
			} else {
				oauthCredential, err = toSecret(opts.secretManager, cfg.OAuth2.ClientSecret, cfg.OAuth2.ClientSecretFile, cfg.OAuth2.ClientSecretRef)
				if err != nil {
					return nil, fmt.Errorf("unable to use client secret: %w", err)
				}
			}
			rt, err = newOAuth2RoundTripper(oauthCredential, cfg.OAuth2, rt, &opts)
			if err != nil {
				return nil, fmt.Errorf("unable to create OAuth2RoundTripper: %w", err)
			}
		}

		if cfg.HTTPHeaders != nil {
			rt = commonconfig.NewHeadersRoundTripper(cfg.HTTPHeaders, rt)
		}

		if opts.userAgent != "" {
			rt = commonconfig.NewUserAgentRoundTripper(opts.userAgent, rt)
		}

		if opts.host != "" {
			rt = commonconfig.NewHostRoundTripper(opts.host, rt)
		}

		// Return a new configured RoundTripper.
		return rt, nil
	}

	tlsConfig, err := opts.newTLSConfigFunc(ctx, &cfg.TLSConfig, commonconfig.WithSecretManager(opts.secretManager))
	if err != nil {
		return nil, err
	}

	tlsSettings, err := roundTripperSettings(&cfg.TLSConfig, opts.secretManager)
	if err != nil {
		return nil, err
	}

	if immutable(&tlsSettings) {
		// No need for a RoundTripper that reloads the files automatically.
		return newRT(tlsConfig)
	}
	return commonconfig.NewTLSRoundTripperWithContext(ctx, tlsConfig, tlsSettings, newRT)
}

type refSecret struct {
	ref     string
	manager commonconfig.SecretManager // manager is expected to be not nil.
}

func (s *refSecret) Fetch(ctx context.Context) (string, error) {
	return s.manager.Fetch(ctx, s.ref)
}

func (s *refSecret) Description() string {
	return "ref " + s.ref
}

func (*refSecret) Immutable() bool {
	return false
}

// toSecret returns a SecretReader from one of the given sources, assuming exactly
// one or none of the sources are provided.
func toSecret(secretManager commonconfig.SecretManager, text commonconfig.Secret, file, ref string) (commonconfig.SecretReader, error) {
	if text != "" {
		return commonconfig.NewInlineSecret(string(text)), nil
	}
	if file != "" {
		return commonconfig.NewFileSecret(file), nil
	}
	if ref != "" {
		if secretManager == nil {
			return nil, errors.New("cannot use secret ref without manager")
		}
		return &refSecret{
			ref:     ref,
			manager: secretManager,
		}, nil
	}
	return nil, nil
}

// copied from prometheus because it is not exported
func roundTripperSettings(c *commonconfig.TLSConfig, secretManager commonconfig.SecretManager) (commonconfig.TLSRoundTripperSettings, error) {
	ca, err := toSecret(secretManager, commonconfig.Secret(c.CA), c.CAFile, c.CARef)
	if err != nil {
		return commonconfig.TLSRoundTripperSettings{}, err
	}
	cert, err := toSecret(secretManager, commonconfig.Secret(c.Cert), c.CertFile, c.CertRef)
	if err != nil {
		return commonconfig.TLSRoundTripperSettings{}, err
	}
	key, err := toSecret(secretManager, c.Key, c.KeyFile, c.KeyRef)
	if err != nil {
		return commonconfig.TLSRoundTripperSettings{}, err
	}
	return commonconfig.TLSRoundTripperSettings{
		CA:   ca,
		Cert: cert,
		Key:  key,
	}, nil
}

// copied from prometheus because it is not exported
func immutable(t *commonconfig.TLSRoundTripperSettings) bool {
	return (t.CA == nil || t.CA.Immutable()) && (t.Cert == nil || t.Cert.Immutable()) && (t.Key == nil || t.Key.Immutable())
}
