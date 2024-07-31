package otelcol

import (
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconfigauth "go.opentelemetry.io/collector/config/configauth"
	otelconfighttp "go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	otelextension "go.opentelemetry.io/collector/extension"
)

// HTTPServerArguments holds shared settings for components which launch HTTP
// servers.
type HTTPServerArguments struct {
	Endpoint string `alloy:"endpoint,attr,optional"`

	TLS *TLSServerArguments `alloy:"tls,block,optional"`

	CORS *CORSArguments `alloy:"cors,block,optional"`

	// TODO(rfratto): auth
	//
	// Figuring out how to do authentication isn't very straightforward here. The
	// auth section links to an authenticator extension.
	//
	// We will need to generally figure out how we want to provide common
	// authentication extensions to all of our components.

	MaxRequestBodySize    units.Base2Bytes `alloy:"max_request_body_size,attr,optional"`
	IncludeMetadata       bool             `alloy:"include_metadata,attr,optional"`
	CompressionAlgorithms []string         `alloy:"compression_algorithms,attr,optional"`
}

var DefaultCompressionAlgorithms = []string{"", "gzip", "zstd", "zlib", "snappy", "deflate"}

func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s...)
}

// Convert converts args into the upstream type.
func (args *HTTPServerArguments) Convert() *otelconfighttp.ServerConfig {
	if args == nil {
		return nil
	}

	return &otelconfighttp.ServerConfig{
		Endpoint:              args.Endpoint,
		TLSSetting:            args.TLS.Convert(),
		CORS:                  args.CORS.Convert(),
		MaxRequestBodySize:    int64(args.MaxRequestBodySize),
		IncludeMetadata:       args.IncludeMetadata,
		CompressionAlgorithms: copyStringSlice(args.CompressionAlgorithms),
	}
}

// CORSArguments holds shared CORS settings for components which launch HTTP
// servers.
type CORSArguments struct {
	AllowedOrigins []string `alloy:"allowed_origins,attr,optional"`
	AllowedHeaders []string `alloy:"allowed_headers,attr,optional"`

	MaxAge int `alloy:"max_age,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *CORSArguments) Convert() *otelconfighttp.CORSConfig {
	if args == nil {
		return nil
	}

	return &otelconfighttp.CORSConfig{
		AllowedOrigins: copyStringSlice(args.AllowedOrigins),
		AllowedHeaders: copyStringSlice(args.AllowedHeaders),

		MaxAge: args.MaxAge,
	}
}

// HTTPClientArguments holds shared HTTP settings for components which launch
// HTTP clients.
type HTTPClientArguments struct {
	Endpoint string `alloy:"endpoint,attr"`

	Compression CompressionType `alloy:"compression,attr,optional"`

	TLS TLSClientArguments `alloy:"tls,block,optional"`

	ReadBufferSize  units.Base2Bytes  `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize units.Base2Bytes  `alloy:"write_buffer_size,attr,optional"`
	Timeout         time.Duration     `alloy:"timeout,attr,optional"`
	Headers         map[string]string `alloy:"headers,attr,optional"`
	// CustomRoundTripper  func(next http.RoundTripper) (http.RoundTripper, error) TODO (@tpaschalis)
	MaxIdleConns         *int           `alloy:"max_idle_conns,attr,optional"`
	MaxIdleConnsPerHost  *int           `alloy:"max_idle_conns_per_host,attr,optional"`
	MaxConnsPerHost      *int           `alloy:"max_conns_per_host,attr,optional"`
	IdleConnTimeout      *time.Duration `alloy:"idle_conn_timeout,attr,optional"`
	DisableKeepAlives    bool           `alloy:"disable_keep_alives,attr,optional"`
	HTTP2ReadIdleTimeout time.Duration  `alloy:"http2_read_idle_timeout,attr,optional"`
	HTTP2PingTimeout     time.Duration  `alloy:"http2_ping_timeout,attr,optional"`

	// Auth is a binding to an otelcol.auth.* component extension which handles
	// authentication.
	Auth *auth.Handler `alloy:"auth,attr,optional"`

	Cookies *Cookies `alloy:"cookies,block,optional"`
}

// Convert converts args into the upstream type.
func (args *HTTPClientArguments) Convert() *otelconfighttp.ClientConfig {
	if args == nil {
		return nil
	}

	// Configure the authentication if args.Auth is set.
	var auth *otelconfigauth.Authentication
	if args.Auth != nil {
		auth = &otelconfigauth.Authentication{AuthenticatorID: args.Auth.ID}
	}

	opaqueHeaders := make(map[string]configopaque.String)
	for headerName, headerVal := range args.Headers {
		opaqueHeaders[headerName] = configopaque.String(headerVal)
	}

	return &otelconfighttp.ClientConfig{
		Endpoint: args.Endpoint,

		Compression: args.Compression.Convert(),

		TLSSetting: *args.TLS.Convert(),

		ReadBufferSize:  int(args.ReadBufferSize),
		WriteBufferSize: int(args.WriteBufferSize),
		Timeout:         args.Timeout,
		Headers:         opaqueHeaders,
		// CustomRoundTripper: func(http.RoundTripper) (http.RoundTripper, error) { panic("not implemented") }, TODO (@tpaschalis)
		MaxIdleConns:         args.MaxIdleConns,
		MaxIdleConnsPerHost:  args.MaxIdleConnsPerHost,
		MaxConnsPerHost:      args.MaxConnsPerHost,
		IdleConnTimeout:      args.IdleConnTimeout,
		DisableKeepAlives:    args.DisableKeepAlives,
		HTTP2ReadIdleTimeout: args.HTTP2ReadIdleTimeout,
		HTTP2PingTimeout:     args.HTTP2PingTimeout,

		Auth: auth,

		Cookies: args.Cookies.Convert(),
	}
}

// Extensions exposes extensions used by args.
func (args *HTTPClientArguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	m := make(map[otelcomponent.ID]otelextension.Extension)
	if args.Auth != nil {
		m[args.Auth.ID] = args.Auth.Extension
	}
	return m
}

type Cookies struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

func (c *Cookies) Convert() *otelconfighttp.CookiesConfig {
	if c == nil {
		return nil
	}

	return &otelconfighttp.CookiesConfig{
		Enabled: c.Enabled,
	}
}
