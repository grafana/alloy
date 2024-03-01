// Package http contains a River serializable definition of the dskit config in
// https://github.com/grafana/dskit/blob/main/server/server.go#L72.
package net

import (
	"flag"
	"math"
	"time"

	dskit "github.com/grafana/dskit/server"
)

const (
	DefaultHTTPPort = 8080

	// using zero as default grpc port to assing random free port when not configured
	DefaultGRPCPort = 0

	// defaults inherited from dskit
	durationInfinity = time.Duration(math.MaxInt64)
	size4MB          = 4 << 20
)

// ServerConfig is a River configuration that allows one to configure a dskit.Server. It
// exposes a subset of the available configurations.
type ServerConfig struct {
	// HTTP configures the HTTP dskit. Note that despite the block being present or not,
	// the dskit is always started.
	HTTP *HTTPConfig `river:"http,block,optional"`

	// GRPC configures the gRPC dskit. Note that despite the block being present or not,
	// the dskit is always started.
	GRPC *GRPCConfig `river:"grpc,block,optional"`

	// GracefulShutdownTimeout configures a timeout to gracefully shut down the server.
	GracefulShutdownTimeout time.Duration `river:"graceful_shutdown_timeout,attr,optional"`
}

// HTTPConfig configures the HTTP dskit started by dskit.Server.
type HTTPConfig struct {
	ListenAddress      string        `river:"listen_address,attr,optional"`
	ListenPort         int           `river:"listen_port,attr,optional"`
	ConnLimit          int           `river:"conn_limit,attr,optional"`
	ServerReadTimeout  time.Duration `river:"server_read_timeout,attr,optional"`
	ServerWriteTimeout time.Duration `river:"server_write_timeout,attr,optional"`
	ServerIdleTimeout  time.Duration `river:"server_idle_timeout,attr,optional"`
}

// Into applies the configs from HTTPConfig into a dskit.Into.
func (h *HTTPConfig) Into(c *dskit.Config) {
	c.HTTPListenAddress = h.ListenAddress
	c.HTTPListenPort = h.ListenPort
	c.HTTPConnLimit = h.ConnLimit
	c.HTTPServerReadTimeout = h.ServerReadTimeout
	c.HTTPServerWriteTimeout = h.ServerWriteTimeout
	c.HTTPServerIdleTimeout = h.ServerIdleTimeout
}

// GRPCConfig configures the gRPC dskit started by dskit.Server.
type GRPCConfig struct {
	ListenAddress              string        `river:"listen_address,attr,optional"`
	ListenPort                 int           `river:"listen_port,attr,optional"`
	ConnLimit                  int           `river:"conn_limit,attr,optional"`
	MaxConnectionAge           time.Duration `river:"max_connection_age,attr,optional"`
	MaxConnectionAgeGrace      time.Duration `river:"max_connection_age_grace,attr,optional"`
	MaxConnectionIdle          time.Duration `river:"max_connection_idle,attr,optional"`
	ServerMaxRecvMsg           int           `river:"server_max_recv_msg_size,attr,optional"`
	ServerMaxSendMsg           int           `river:"server_max_send_msg_size,attr,optional"`
	ServerMaxConcurrentStreams uint          `river:"server_max_concurrent_streams,attr,optional"`
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
}

// Convert converts the River-based ServerConfig into a dskit.Config object.
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
