// Package config contains types from github.com/prometheus/common/config,
// but modifies them to be serializable with Alloy.
package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/common/config"
)

const bearerAuth string = "Bearer"

// HTTPClientConfig mirrors config.HTTPClientConfig
type HTTPClientConfig struct {
	BasicAuth       *BasicAuth        `alloy:"basic_auth,block,optional"`
	Authorization   *Authorization    `alloy:"authorization,block,optional"`
	OAuth2          *OAuth2Config     `alloy:"oauth2,block,optional"`
	BearerToken     alloytypes.Secret `alloy:"bearer_token,attr,optional"`
	BearerTokenFile string            `alloy:"bearer_token_file,attr,optional"`
	ProxyConfig     *ProxyConfig      `alloy:",squash"`
	TLSConfig       TLSConfig         `alloy:"tls_config,block,optional"`
	FollowRedirects bool              `alloy:"follow_redirects,attr,optional"`
	EnableHTTP2     bool              `alloy:"enable_http2,attr,optional"`
}

// SetToDefault implements the syntax.Defaulter
func (h *HTTPClientConfig) SetToDefault() {
	*h = DefaultHTTPClientConfig
}

// Validate returns an error if h is invalid.
func (h *HTTPClientConfig) Validate() error {
	if h == nil {
		return nil
	}

	authCount := 0
	if h.BasicAuth != nil {
		authCount++
	}
	if h.Authorization != nil {
		authCount++
	}
	if h.OAuth2 != nil {
		authCount++
	}
	if len(h.BearerToken) > 0 {
		authCount++
	}
	if len(h.BearerTokenFile) > 0 {
		authCount++
	}

	if authCount > 1 {
		return fmt.Errorf("at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
	}

	// TODO: Validate should not be modifying the object
	if len(h.BearerToken) > 0 {
		h.Authorization = &Authorization{Credentials: h.BearerToken}
		h.Authorization.Type = bearerAuth
		h.BearerToken = ""
	}

	// TODO: Validate should not be modifying the object
	if len(h.BearerTokenFile) > 0 {
		h.Authorization = &Authorization{CredentialsFile: h.BearerTokenFile}
		h.Authorization.Type = bearerAuth
		h.BearerTokenFile = ""
	}

	return h.ProxyConfig.Validate()
}

// Convert converts HTTPClientConfig to the native Prometheus type. If h is
// nil, the default client config is returned.
func (h *HTTPClientConfig) Convert() *config.HTTPClientConfig {
	if h == nil {
		return &config.DefaultHTTPClientConfig
	}

	return &config.HTTPClientConfig{
		BasicAuth:       h.BasicAuth.Convert(),
		Authorization:   h.Authorization.Convert(),
		OAuth2:          h.OAuth2.Convert(),
		BearerToken:     config.Secret(h.BearerToken),
		BearerTokenFile: h.BearerTokenFile,
		TLSConfig:       *h.TLSConfig.Convert(),
		FollowRedirects: h.FollowRedirects,
		EnableHTTP2:     h.EnableHTTP2,
		ProxyConfig:     h.ProxyConfig.Convert(),
	}
}

// Clone creates a shallow clone of h.
func CloneDefaultHTTPClientConfig() *HTTPClientConfig {
	clone := DefaultHTTPClientConfig
	return &clone
}

// DefaultHTTPClientConfig for initializing objects
var DefaultHTTPClientConfig = HTTPClientConfig{
	FollowRedirects: true,
	EnableHTTP2:     true,
}

// BasicAuth configures Basic HTTP authentication credentials.
type BasicAuth struct {
	Username     string            `alloy:"username,attr,optional"`
	Password     alloytypes.Secret `alloy:"password,attr,optional"`
	PasswordFile string            `alloy:"password_file,attr,optional"`
}

// Convert converts our type to the native prometheus type
func (b *BasicAuth) Convert() *config.BasicAuth {
	if b == nil {
		return nil
	}
	return &config.BasicAuth{
		Username:     b.Username,
		Password:     config.Secret(b.Password),
		PasswordFile: b.PasswordFile,
	}
}

func (b *BasicAuth) Validate() error {
	if b == nil {
		return nil
	}

	if string(b.Password) != "" && b.PasswordFile != "" {
		return fmt.Errorf("at most one of basic_auth password & password_file must be configured")
	}

	return nil
}

type ProxyConfig struct {
	ProxyURL             URL         `alloy:"proxy_url,attr,optional"`
	NoProxy              string      `alloy:"no_proxy,attr,optional"`
	ProxyFromEnvironment bool        `alloy:"proxy_from_environment,attr,optional"`
	ProxyConnectHeader   ProxyHeader `alloy:",squash"`
}

func (p *ProxyConfig) Convert() config.ProxyConfig {
	if p == nil {
		return config.ProxyConfig{}
	}

	return config.ProxyConfig{
		ProxyURL:             p.ProxyURL.Convert(),
		NoProxy:              p.NoProxy,
		ProxyFromEnvironment: p.ProxyFromEnvironment,
		ProxyConnectHeader:   p.ProxyConnectHeader.Convert(),
	}
}

func (p *ProxyConfig) Validate() error {
	if p == nil {
		return nil
	}

	if len(p.ProxyConnectHeader.Header) > 0 && (!p.ProxyFromEnvironment && (p.ProxyURL.URL == nil || p.ProxyURL.String() == "")) {
		return fmt.Errorf("if proxy_connect_header is configured, proxy_url or proxy_from_environment must also be configured")
	}
	if p.ProxyFromEnvironment && p.ProxyURL.URL != nil && p.ProxyURL.String() != "" {
		return fmt.Errorf("if proxy_from_environment is configured, proxy_url must not be configured")
	}
	if p.ProxyFromEnvironment && p.NoProxy != "" {
		return fmt.Errorf("if proxy_from_environment is configured, no_proxy must not be configured")
	}
	if p.ProxyURL.URL == nil && p.NoProxy != "" {
		return fmt.Errorf("if no_proxy is configured, proxy_url must also be configured")
	}

	return nil
}

// URL mirrors config.URL
type URL struct {
	*url.URL
}

// MarshalText implements encoding.TextMarshaler
func (u URL) MarshalText() (text []byte, err error) {
	u2 := &config.URL{
		URL: u.URL,
	}
	if u.URL != nil {
		return []byte(u2.Redacted()), nil
	}
	return nil, nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (u *URL) UnmarshalText(text []byte) error {
	s := string(text)
	urlp, err := url.Parse(s)
	if err != nil {
		return err
	}
	u.URL = urlp
	return nil
}

// Convert converts our type to the native prometheus type
func (u *URL) Convert() config.URL {
	if u == nil {
		return config.URL{URL: nil}
	}
	return config.URL{URL: u.URL}
}

type ProxyHeader struct {
	Header map[string][]alloytypes.Secret `alloy:"proxy_connect_header,attr,optional"`
}

func (h *ProxyHeader) Convert() config.ProxyHeader {
	if h == nil {
		return nil
	}
	header := make(config.ProxyHeader)
	for name, values := range h.Header {
		var s []config.Secret
		if values != nil {
			s = make([]config.Secret, 0, len(values))
			for _, value := range values {
				s = append(s, config.Secret(value))
			}
		}
		header[name] = s
	}
	return header
}

// Authorization sets up HTTP authorization credentials.
type Authorization struct {
	Type            string            `alloy:"type,attr,optional"`
	Credentials     alloytypes.Secret `alloy:"credentials,attr,optional"`
	CredentialsFile string            `alloy:"credentials_file,attr,optional"`
}

// Convert converts our type to the native prometheus type
func (a *Authorization) Convert() *config.Authorization {
	if a == nil {
		return nil
	}
	return &config.Authorization{
		Type:            a.Type,
		Credentials:     config.Secret(a.Credentials),
		CredentialsFile: a.CredentialsFile,
	}
}

func (a *Authorization) Validate() error {
	if a == nil {
		return nil
	}

	if string(a.Credentials) != "" && a.CredentialsFile != "" {
		return fmt.Errorf("at most one of authorization credentials & credentials_file must be configured")
	}

	// TODO: Validate should not be modifying the object
	a.Type = strings.TrimSpace(a.Type)
	if len(a.Type) == 0 {
		a.Type = bearerAuth
	}

	if strings.ToLower(a.Type) == "basic" {
		return fmt.Errorf(`authorization type cannot be set to "basic", use "basic_auth" block instead`)
	}

	return nil
}

// TLSVersion mirrors config.TLSVersion
type TLSVersion uint16

// MarshalText implements encoding.TextMarshaler
func (tv TLSVersion) MarshalText() (text []byte, err error) {
	for s, v := range config.TLSVersions {
		if config.TLSVersion(tv) == v {
			return []byte(s), nil
		}
	}
	return nil, fmt.Errorf("unknown TLS version: %d", tv)
}

// UnmarshalText implements encoding.TextUnmarshaler
func (tv *TLSVersion) UnmarshalText(text []byte) error {
	if v, ok := config.TLSVersions[string(text)]; ok {
		*tv = TLSVersion(v)
		return nil
	}
	return fmt.Errorf("unknown TLS version: %s", string(text))
}

// TLSConfig sets up options for TLS connections.
type TLSConfig struct {
	CA                 string            `alloy:"ca_pem,attr,optional"`
	CAFile             string            `alloy:"ca_file,attr,optional"`
	Cert               string            `alloy:"cert_pem,attr,optional"`
	CertFile           string            `alloy:"cert_file,attr,optional"`
	Key                alloytypes.Secret `alloy:"key_pem,attr,optional"`
	KeyFile            string            `alloy:"key_file,attr,optional"`
	ServerName         string            `alloy:"server_name,attr,optional"`
	InsecureSkipVerify bool              `alloy:"insecure_skip_verify,attr,optional"`
	MinVersion         TLSVersion        `alloy:"min_version,attr,optional"`
}

// Convert converts our type to the native prometheus type
func (t *TLSConfig) Convert() *config.TLSConfig {
	if t == nil {
		return nil
	}
	return &config.TLSConfig{
		CA:                 t.CA,
		CAFile:             t.CAFile,
		Cert:               t.Cert,
		CertFile:           t.CertFile,
		Key:                config.Secret(t.Key),
		KeyFile:            t.KeyFile,
		ServerName:         t.ServerName,
		InsecureSkipVerify: t.InsecureSkipVerify,
		MinVersion:         config.TLSVersion(t.MinVersion),
	}
}

// Validate reports whether t is valid.
func (t *TLSConfig) Validate() error {
	if len(t.CA) > 0 && len(t.CAFile) > 0 {
		return fmt.Errorf("at most one of ca_pem and ca_file must be configured")
	}
	if len(t.Cert) > 0 && len(t.CertFile) > 0 {
		return fmt.Errorf("at most one of cert_pem and cert_file must be configured")
	}
	if len(t.Key) > 0 && len(t.KeyFile) > 0 {
		return fmt.Errorf("at most one of key_pem and key_file must be configured")
	}

	var (
		usingClientCert = len(t.Cert) > 0 || len(t.CertFile) > 0
		usingClientKey  = len(t.Key) > 0 || len(t.KeyFile) > 0
	)

	if usingClientCert && !usingClientKey {
		return fmt.Errorf("exactly one of key_pem or key_file must be configured when a client certificate is configured")
	} else if usingClientKey && !usingClientCert {
		return fmt.Errorf("exactly one of cert_pem or cert_file must be configured when a client key is configured")
	}

	return nil
}

// OAuth2Config sets up the OAuth2 client.
type OAuth2Config struct {
	ClientID         string            `alloy:"client_id,attr,optional"`
	ClientSecret     alloytypes.Secret `alloy:"client_secret,attr,optional"`
	ClientSecretFile string            `alloy:"client_secret_file,attr,optional"`
	Scopes           []string          `alloy:"scopes,attr,optional"`
	TokenURL         string            `alloy:"token_url,attr,optional"`
	EndpointParams   map[string]string `alloy:"endpoint_params,attr,optional"`
	ProxyConfig      *ProxyConfig      `alloy:",squash"`
	TLSConfig        *TLSConfig        `alloy:"tls_config,block,optional"`
}

// Convert converts our type to the native prometheus type
func (o *OAuth2Config) Convert() *config.OAuth2 {
	if o == nil {
		return nil
	}
	oa := &config.OAuth2{
		ClientID:         o.ClientID,
		ClientSecret:     config.Secret(o.ClientSecret),
		ClientSecretFile: o.ClientSecretFile,
		Scopes:           o.Scopes,
		TokenURL:         o.TokenURL,
		EndpointParams:   o.EndpointParams,
		ProxyConfig:      o.ProxyConfig.Convert(),
	}
	if o.TLSConfig != nil {
		oa.TLSConfig = *o.TLSConfig.Convert()
	}
	return oa
}

func (o *OAuth2Config) Validate() error {
	if o == nil {
		return nil
	}

	if len(o.ClientID) == 0 {
		return fmt.Errorf("oauth2 client_id must be configured")
	}
	if len(o.ClientSecret) == 0 && len(o.ClientSecretFile) == 0 {
		return fmt.Errorf("either oauth2 client_secret or client_secret_file must be configured")
	}
	if len(o.TokenURL) == 0 {
		return fmt.Errorf("oauth2 token_url must be configured")
	}
	if len(o.ClientSecret) > 0 && len(o.ClientSecretFile) > 0 {
		return fmt.Errorf("at most one of oauth2 client_secret & client_secret_file must be configured")
	}

	return o.ProxyConfig.Validate()
}

type SysLogFormat string

const (
	// A modern Syslog RFC
	SyslogFormatRFC5424 SysLogFormat = "rfc5424"
	// A legacy Syslog RFC also known as BSD-syslog
	SyslogFormatRFC3164 SysLogFormat = "rfc3164"
)

// MarshalText implements encoding.TextMarshaler
func (s SysLogFormat) MarshalText() (text []byte, err error) {
	return []byte(s), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (s *SysLogFormat) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case "rfc5424":
		*s = SyslogFormatRFC5424
	case "rfc3164":
		*s = SyslogFormatRFC3164
	default:
		return fmt.Errorf("unknown syslog format: %s", str)
	}

	return nil
}
