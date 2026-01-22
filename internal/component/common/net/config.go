// Package http contains a Alloy-serializable definition of the dskit config in
// https://github.com/grafana/dskit/blob/main/server/server.go#L72.
package net

import (
	"flag"
	"math"
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	dskit "github.com/grafana/dskit/server"
	"github.com/prometheus/common/config"
	"golang.org/x/net/http2"
)

const (
	DefaultHTTPPort = 8080

	// using zero as default grpc port to assing random free port when not configured
	DefaultGRPCPort = 0

	// defaults inherited from dskit
	durationInfinity = time.Duration(math.MaxInt64)
	size4MB          = 4 << 20
)

// ServerConfig is an Alloy configuration that allows one to configure a dskit.Server. It
// exposes a subset of the available configurations.
type ServerConfig struct {
	// HTTP configures the HTTP dskit. Note that despite the block being present or not,
	// the dskit is always started.
	HTTP *HTTPConfig `alloy:"http,block,optional"`

	// GRPC configures the gRPC dskit. Note that despite the block being present or not,
	// the dskit is always started.
	GRPC *GRPCConfig `alloy:"grpc,block,optional"`

	// GracefulShutdownTimeout configures a timeout to gracefully shut down the server.
	GracefulShutdownTimeout time.Duration `alloy:"graceful_shutdown_timeout,attr,optional"`
}

// HTTPConfig configures the HTTP dskit started by dskit.Server.
type HTTPConfig struct {
	ListenAddress      string        `alloy:"listen_address,attr,optional"`
	ListenPort         int           `alloy:"listen_port,attr,optional"`
	ConnLimit          int           `alloy:"conn_limit,attr,optional"`
	ServerReadTimeout  time.Duration `alloy:"server_read_timeout,attr,optional"`
	ServerWriteTimeout time.Duration `alloy:"server_write_timeout,attr,optional"`
	ServerIdleTimeout  time.Duration `alloy:"server_idle_timeout,attr,optional"`
	TLSConfig          *TLSConfig    `alloy:"tls,block,optional"`
	HTTP2              *HTTP2Config  `alloy:"http2,block,optional"`
}

// Into applies the configs from HTTPConfig into a dskit.Into.
func (h *HTTPConfig) Into(c *dskit.Config) {
	c.HTTPListenAddress = h.ListenAddress
	c.HTTPListenPort = h.ListenPort
	c.HTTPConnLimit = h.ConnLimit
	c.HTTPServerReadTimeout = h.ServerReadTimeout
	c.HTTPServerWriteTimeout = h.ServerWriteTimeout
	c.HTTPServerIdleTimeout = h.ServerIdleTimeout
	if h.TLSConfig != nil {
		h.TLSConfig.Into(&c.HTTPTLSConfig)
		if h.TLSConfig.MinVersion != c.MinVersion {
			c.MinVersion = h.TLSConfig.MinVersion
		}
		if h.TLSConfig.CipherSuites != c.CipherSuites {
			c.CipherSuites = h.TLSConfig.CipherSuites
		}
	}
}

// GRPCConfig configures the gRPC dskit started by dskit.Server.
type GRPCConfig struct {
	ListenAddress              string        `alloy:"listen_address,attr,optional"`
	ListenPort                 int           `alloy:"listen_port,attr,optional"`
	ConnLimit                  int           `alloy:"conn_limit,attr,optional"`
	MaxConnectionAge           time.Duration `alloy:"max_connection_age,attr,optional"`
	MaxConnectionAgeGrace      time.Duration `alloy:"max_connection_age_grace,attr,optional"`
	MaxConnectionIdle          time.Duration `alloy:"max_connection_idle,attr,optional"`
	ServerMaxRecvMsg           int           `alloy:"server_max_recv_msg_size,attr,optional"`
	ServerMaxSendMsg           int           `alloy:"server_max_send_msg_size,attr,optional"`
	ServerMaxConcurrentStreams uint          `alloy:"server_max_concurrent_streams,attr,optional"`
	TLSConfig                  *TLSConfig    `alloy:"tls,block,optional"`
}

// TLSConfig sets up options for TLS connections.
type TLSConfig struct {
	Cert         string            `alloy:"cert_pem,attr,optional"`
	Key          alloytypes.Secret `alloy:"key_pem,attr,optional"`
	CertFile     string            `alloy:"cert_file,attr,optional"`
	KeyFile      string            `alloy:"key_file,attr,optional"`
	ClientAuth   string            `alloy:"client_auth_type,attr,optional"`
	ClientCAFile string            `alloy:"client_ca_file,attr,optional"`
	ClientCA     string            `alloy:"client_ca,attr,optional"`
	CipherSuites string            `alloy:"cipher_suites,attr,optional"`
	MinVersion   string            `alloy:"min_version,attr,optional"`
}

type HTTP2Config struct {
	Enabled                      bool          `alloy:"enabled,attr,optional"`
	MaxHandlers                  int           `alloy:"max_handlers,attr,optional"`
	MaxConcurrentStreams         uint32        `alloy:"max_concurrent_streams,attr,optional"`
	MaxDecoderHeaderTableSize    uint32        `alloy:"max_decoder_header_table_size,attr,optional"`
	MaxEncoderHeaderTableSize    uint32        `alloy:"max_encoder_header_table_size,attr,optional"`
	MaxReadFrameSize             uint32        `alloy:"max_read_frame_size,attr,optional"`
	PermitProhibitedCipherSuites bool          `alloy:"permit_prohibited_ciphers,attr,optional"`
	IdleTimeout                  time.Duration `alloy:"idle_timeout,attr,optional"`
	ReadIdleTimeout              time.Duration `alloy:"read_idle_timeout,attr,optional"`
	PingTimeout                  time.Duration `alloy:"ping_timeout,attr,optional"`
	WriteByteTimeout             time.Duration `alloy:"write_byte_timeout,attr,optional"`
	MaxUploadBufferPerConnection int32         `alloy:"max_upload_buffer_per_connection,attr,optional"`
	MaxUploadBufferPerStream     int32         `alloy:"max_upload_buffer_per_stream,attr,optional"`
}

func (c *HTTP2Config) Server() *http2.Server {
	if c == nil || !c.Enabled {
		return nil
	}
	return &http2.Server{
		MaxHandlers:                  c.MaxHandlers,
		MaxConcurrentStreams:         c.MaxConcurrentStreams,
		MaxDecoderHeaderTableSize:    c.MaxDecoderHeaderTableSize,
		MaxEncoderHeaderTableSize:    c.MaxEncoderHeaderTableSize,
		MaxReadFrameSize:             c.MaxReadFrameSize,
		PermitProhibitedCipherSuites: c.PermitProhibitedCipherSuites,
		IdleTimeout:                  c.IdleTimeout,
		ReadIdleTimeout:              c.ReadIdleTimeout,
		PingTimeout:                  c.PingTimeout,
		WriteByteTimeout:             c.WriteByteTimeout,
		MaxUploadBufferPerConnection: c.MaxUploadBufferPerConnection,
		MaxUploadBufferPerStream:     c.MaxUploadBufferPerStream,
	}
}

// Into applies the configs from GRPCConfig into a dskit.Into.
func (g *GRPCConfig) Into(c *dskit.Config) {
	c.GRPCListenAddress = g.ListenAddress
	c.GRPCListenPort = g.ListenPort
	c.GRPCConnLimit = g.ConnLimit
	c.GRPCServerMaxConnectionAge = g.MaxConnectionAge
	c.GRPCServerMaxConnectionAgeGrace = g.MaxConnectionAgeGrace
	c.GRPCServerMaxConnectionIdle = g.MaxConnectionIdle
	c.GRPCServerMaxRecvMsgSize = g.ServerMaxRecvMsg
	c.GRPCServerMaxSendMsgSize = g.ServerMaxSendMsg
	c.GRPCServerMaxConcurrentStreams = g.ServerMaxConcurrentStreams
	if g.TLSConfig != nil {
		g.TLSConfig.Into(&c.GRPCTLSConfig)
		if g.TLSConfig.MinVersion != c.MinVersion {
			c.MinVersion = g.TLSConfig.MinVersion
		}
		if g.TLSConfig.CipherSuites != c.CipherSuites {
			c.CipherSuites = g.TLSConfig.CipherSuites
		}
	}
}

// Into applies the configs from TLSConfig to dskit.TLSConfig
func (t *TLSConfig) Into(c *dskit.TLSConfig) {
	c.TLSCert = t.Cert
	c.TLSKey = config.Secret(t.Key)
	c.TLSCertPath = t.CertFile
	c.TLSKeyPath = t.KeyFile
	c.ClientCAs = t.ClientCAFile
	c.ClientCAsText = t.ClientCA
	c.ClientAuth = t.ClientAuth
}

// Convert converts the Alloy-based ServerConfig into a dskit.Config object.
func (c *ServerConfig) convert() dskit.Config {
	cfg := newdskitDefaultConfig()
	// use the configured http/grpc blocks, and if not, use a mixin of our defaults, and
	// dskit's as a fallback
	if c.HTTP != nil {
		c.HTTP.Into(&cfg)
	} else {
		DefaultServerConfig().HTTP.Into(&cfg)
	}
	if c.GRPC != nil {
		c.GRPC.Into(&cfg)
	} else {
		DefaultServerConfig().GRPC.Into(&cfg)
	}
	cfg.ServerGracefulShutdownTimeout = c.GracefulShutdownTimeout
	return cfg
}

// newdskitDefaultConfig creates a new dskit.Config object with some overridden defaults.
func newdskitDefaultConfig() dskit.Config {
	c := dskit.Config{}
	c.RegisterFlags(flag.NewFlagSet("empty", flag.ContinueOnError))
	// By default, do not register instrumentation since every metric is later registered
	// inside a custom register
	c.RegisterInstrumentation = false
	c.GRPCCollectMaxStreamsByConn = false
	return c
}

// DefaultServerConfig creates a new ServerConfig with defaults applied. Note that some are inherited from
// dskit, but copied in our config model to make the mixin logic simpler.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		HTTP: &HTTPConfig{
			ListenAddress:      "",
			ListenPort:         DefaultHTTPPort,
			ConnLimit:          0,
			ServerReadTimeout:  30 * time.Second,
			ServerWriteTimeout: 30 * time.Second,
			ServerIdleTimeout:  120 * time.Second,
			HTTP2: &HTTP2Config{
				Enabled:                      false,
				MaxHandlers:                  0,
				MaxConcurrentStreams:         100,
				MaxDecoderHeaderTableSize:    4096,
				MaxEncoderHeaderTableSize:    4096,
				MaxReadFrameSize:             0,
				PermitProhibitedCipherSuites: false,
				IdleTimeout:                  0,
				ReadIdleTimeout:              0,
				PingTimeout:                  15 * time.Second,
				WriteByteTimeout:             0,
				MaxUploadBufferPerConnection: 0,
				MaxUploadBufferPerStream:     0,
			},
		},
		GRPC: &GRPCConfig{
			ListenAddress:              "",
			ListenPort:                 DefaultGRPCPort,
			ConnLimit:                  0,
			MaxConnectionAge:           durationInfinity,
			MaxConnectionAgeGrace:      durationInfinity,
			MaxConnectionIdle:          durationInfinity,
			ServerMaxConcurrentStreams: 100,
			ServerMaxSendMsg:           size4MB,
			ServerMaxRecvMsg:           size4MB,
		},
		GracefulShutdownTimeout: 30 * time.Second,
	}
}
