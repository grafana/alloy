package otelcol

import (
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconfigauth "go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configcompression"
	otelconfighttp "go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
)

// HTTPServerArguments holds shared settings for components which launch HTTP
// servers.
type HTTPServerArguments struct {
	Endpoint string `alloy:"endpoint,attr,optional"`

	TLS *TLSServerArguments `alloy:"tls,block,optional"`

	CORS *CORSArguments `alloy:"cors,block,optional"`

	// Auth is a binding to an otelcol.auth.* component extension which handles
	// authentication.
	// alloy name is auth instead of authentication so the user interface is the same as exporter components.
	Authentication *auth.Handler `alloy:"auth,attr,optional"`

	MaxRequestBodySize    units.Base2Bytes `alloy:"max_request_body_size,attr,optional"`
	IncludeMetadata       bool             `alloy:"include_metadata,attr,optional"`
	CompressionAlgorithms []string         `alloy:"compression_algorithms,attr,optional"`

	KeepAlivesEnabled *bool `alloy:"keep_alives_enabled,attr,optional"`
}

var DefaultCompressionAlgorithms = []string{"", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"}

func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s...)
}

// Convert converts args into the upstream type.
func (args *HTTPServerArguments) Convert() (configoptional.Optional[otelconfighttp.ServerConfig], error) {
	if args == nil {
		return configoptional.None[otelconfighttp.ServerConfig](), nil
	}

	// If auth is set by the user retrieve the associated extension from the handler.
	// if the extension does not support server auth an error will be returned.
	var authentication configoptional.Optional[otelconfighttp.AuthConfig]
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Server)
		if err != nil {
			return configoptional.None[otelconfighttp.ServerConfig](), err
		}

		authentication = configoptional.Some(otelconfighttp.AuthConfig{
			Config: otelconfigauth.Config{
				AuthenticatorID: ext.ID,
			},
		})
	}

	// Default true boolean
	keepAliveEnabled := true
	if args.KeepAlivesEnabled != nil {
		keepAliveEnabled = *args.KeepAlivesEnabled
	}

	return configoptional.Some(otelconfighttp.ServerConfig{
		Endpoint:              args.Endpoint,
		TLS:                   args.TLS.Convert(),
		KeepAlivesEnabled:     keepAliveEnabled,
		CORS:                  args.CORS.Convert(),
		MaxRequestBodySize:    int64(args.MaxRequestBodySize),
		IncludeMetadata:       args.IncludeMetadata,
		CompressionAlgorithms: copyStringSlice(args.CompressionAlgorithms),
		Auth:                  authentication,
	}), nil
}

// Temporary function until all upstream components are converted to use configoptional.Optional.
func (args *HTTPServerArguments) ConvertToPtr() (*otelconfighttp.ServerConfig, error) {
	converted, err := args.Convert()
	return converted.Get(), err
}

// Extensions exposes extensions used by args.
func (args *HTTPServerArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Server)
		// Extension will not be registered if there was an error.
		if err != nil {
			return m
		}
		m[ext.ID] = ext.Extension
	}
	return m
}

// CORSArguments holds shared CORS settings for components which launch HTTP
// servers.
type CORSArguments struct {
	AllowedOrigins []string `alloy:"allowed_origins,attr,optional"`
	AllowedHeaders []string `alloy:"allowed_headers,attr,optional"`

	MaxAge int `alloy:"max_age,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *CORSArguments) Convert() configoptional.Optional[otelconfighttp.CORSConfig] {
	if args == nil {
		return configoptional.None[otelconfighttp.CORSConfig]()
	}

	return configoptional.Some(otelconfighttp.CORSConfig{
		AllowedOrigins: copyStringSlice(args.AllowedOrigins),
		AllowedHeaders: copyStringSlice(args.AllowedHeaders),
		MaxAge:         args.MaxAge,
	})
}

// HTTPClientArguments holds shared HTTP settings for components which launch
// HTTP clients.
type HTTPClientArguments struct {
	Endpoint string `alloy:"endpoint,attr"`

	ProxyUrl string `alloy:"proxy_url,attr,optional"`

	Compression       CompressionType    `alloy:"compression,attr,optional"`
	CompressionParams *CompressionParams `alloy:"compression_params,block,optional"`

	TLS TLSClientArguments `alloy:"tls,block,optional"`

	ReadBufferSize       units.Base2Bytes  `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize      units.Base2Bytes  `alloy:"write_buffer_size,attr,optional"`
	Timeout              time.Duration     `alloy:"timeout,attr,optional"`
	Headers              map[string]string `alloy:"headers,attr,optional"`
	MaxIdleConns         int               `alloy:"max_idle_conns,attr,optional"`
	MaxIdleConnsPerHost  int               `alloy:"max_idle_conns_per_host,attr,optional"`
	MaxConnsPerHost      int               `alloy:"max_conns_per_host,attr,optional"`
	IdleConnTimeout      time.Duration     `alloy:"idle_conn_timeout,attr,optional"`
	DisableKeepAlives    bool              `alloy:"disable_keep_alives,attr,optional"`
	HTTP2ReadIdleTimeout time.Duration     `alloy:"http2_read_idle_timeout,attr,optional"`
	HTTP2PingTimeout     time.Duration     `alloy:"http2_ping_timeout,attr,optional"`
	ForceAttemptHTTP2    bool              `alloy:"force_attempt_http2,attr,optional"`

	// Auth is a binding to an otelcol.auth.* component extension which handles
	// authentication.
	// alloy name is auth instead of authentication to not break user interface compatibility.
	Authentication *auth.Handler `alloy:"auth,attr,optional"`

	Cookies *Cookies `alloy:"cookies,block,optional"`
}

// Convert converts args into the upstream type.
func (args *HTTPClientArguments) Convert() (*otelconfighttp.ClientConfig, error) {
	if args == nil {
		return nil, nil
	}

	// Configure the authentication if args.Auth is set.
	var authentication configoptional.Optional[otelconfigauth.Config]
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Client)
		if err != nil {
			return nil, err
		}
		authentication = configoptional.Some(otelconfigauth.Config{AuthenticatorID: ext.ID})
	}

	opaqueHeaders := configopaque.MapList{}
	for headerName, headerVal := range args.Headers {
		opaqueHeaders.Set(headerName, configopaque.String(headerVal))
	}

	v := otelconfighttp.ClientConfig{
		Endpoint: args.Endpoint,

		ProxyURL: args.ProxyUrl,

		Compression: args.Compression.Convert(),

		TLS: *args.TLS.Convert(),

		ReadBufferSize:       int(args.ReadBufferSize),
		WriteBufferSize:      int(args.WriteBufferSize),
		Timeout:              args.Timeout,
		Headers:              opaqueHeaders,
		MaxIdleConns:         args.MaxIdleConns,
		MaxIdleConnsPerHost:  args.MaxIdleConnsPerHost,
		MaxConnsPerHost:      args.MaxConnsPerHost,
		IdleConnTimeout:      args.IdleConnTimeout,
		DisableKeepAlives:    args.DisableKeepAlives,
		HTTP2ReadIdleTimeout: args.HTTP2ReadIdleTimeout,
		HTTP2PingTimeout:     args.HTTP2PingTimeout,
		ForceAttemptHTTP2:    args.ForceAttemptHTTP2,

		Auth: authentication,

		Cookies: args.Cookies.Convert(),
	}

	if args.CompressionParams != nil {
		v.CompressionParams = *args.CompressionParams.Convert()
	}

	return &v, nil
}

// Extensions exposes extensions used by args.
func (args *HTTPClientArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Authentication != nil {
		ext, err := args.Authentication.GetExtension(auth.Client)
		if err != nil {
			return m
		}
		m[ext.ID] = ext.Extension
	}
	return m
}

type Cookies struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

func (c *Cookies) Convert() configoptional.Optional[otelconfighttp.CookiesConfig] {
	if c == nil {
		return configoptional.None[otelconfighttp.CookiesConfig]()
	}

	return configoptional.Some(otelconfighttp.CookiesConfig{})
}

type CompressionParams struct {
	Level int `alloy:"level,attr"`
}

func (c *CompressionParams) Convert() *configcompression.CompressionParams {
	if c == nil {
		return nil
	}

	return &configcompression.CompressionParams{
		Level: configcompression.Level(c.Level),
	}
}
