package net

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	dskit "github.com/grafana/dskit/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/slogadapter"
)

// TargetServer is wrapper around dskit.Server that handles some common
// configuration used in all components that expose a network server. It just
// handles configuration and initialization, the handlers implementation are
// left to the consumer.
type TargetServer struct {
	logger           *slog.Logger
	config           *dskit.Config
	metricsNamespace string
	server           *dskit.Server
	http2            *HTTP2Config
}

// NewTargetServer creates a new TargetServer, applying some defaults to the server configuration.
// If provided config is nil, a default configuration will be used instead.
func NewTargetServer(logger *slog.Logger, metricsNamespace string, reg prometheus.Registerer, config *ServerConfig) (*TargetServer, error) {
	// TODO: add support for different validation schemes.
	//nolint:staticcheck
	if !model.IsValidMetricName(model.LabelValue(metricsNamespace)) {
		return nil, fmt.Errorf("metrics namespace is not prometheus compatible: %s", metricsNamespace)
	}

	ts := &TargetServer{
		logger:           logger,
		metricsNamespace: metricsNamespace,
	}

	if config == nil {
		config = DefaultServerConfig()
	}
	if config.HTTP != nil {
		ts.http2 = config.HTTP.HTTP2
	}

	// convert from Alloy into the dskit config
	serverCfg := config.convert()
	// Set the config to the new combined config.
	// Avoid logging entire received request on failures
	serverCfg.ExcludeRequestInLog = true
	// Configure dedicated metrics registerer
	serverCfg.Registerer = reg
	// Persist crafter config in server
	ts.config = &serverCfg
	// To prevent metric collisions because all metrics are going to be registered in the global Prometheus registry.
	ts.config.MetricsNamespace = ts.metricsNamespace
	// We don't want the /debug and /metrics endpoints running, since this is not
	// the main HTTP server. We want this target to expose the least surface area
	// possible, hence disabling dskit HTTP server metrics and debugging
	// functionality.
	ts.config.RegisterInstrumentation = false
	// Add logger to dskit
	ts.config.Log = slogadapter.GoKit(ts.logger.Handler())

	return ts, nil
}

// MountAndRun mounts the handlers and starting the server.
func (ts *TargetServer) MountAndRun(mountRoute func(router *mux.Router)) error {
	ts.logger.Info("starting server")
	srv, err := dskit.New(*ts.config)
	if err != nil {
		return err
	}

	ts.server = srv

	if http2Config := ts.http2.Server(); http2Config != nil {
		ts.server.HTTPServer.HTTP2 = http2Config
		protocols := new(http.Protocols)
		protocols.SetHTTP1(true)
		protocols.SetHTTP2(true)
		protocols.SetUnencryptedHTTP2(true)
		ts.server.HTTPServer.Protocols = protocols
		if ts.http2.MaxHandlers != 0 {
			ts.logger.Warn("http2 max_handlers is deprecated and has no effect; remove it from the configuration", "max_handlers", ts.http2.MaxHandlers)
		}
		if ts.http2.IdleTimeout != 0 {
			// net/http uses one idle timeout for HTTP/1 and HTTP/2.
			ts.server.HTTPServer.IdleTimeout = ts.http2.IdleTimeout
			ts.logger.Warn("http2 idle_timeout is deprecated; use server_idle_timeout instead; the timeout now applies to both HTTP/1 and HTTP/2", "idle_timeout", ts.http2.IdleTimeout)
		}
	}
	mountRoute(ts.server.HTTP)

	go func() {
		err := srv.Run()
		if err != nil {
			ts.logger.Error("server shutdown with error", "err", err)
		}
	}()

	return nil
}

// HTTPListenAddr returns the listen address of the HTTP server.
func (ts *TargetServer) HTTPListenAddr() string {
	return ts.server.HTTPListenAddr().String()
}

// GRPCListenAddr returns the listen address of the gRPC server.
func (ts *TargetServer) GRPCListenAddr() string {
	return ts.server.GRPCListenAddr().String()
}

// StopAndShutdown stops and shuts down the underlying server.
func (ts *TargetServer) StopAndShutdown() {
	ts.server.Stop()
	ts.server.Shutdown()
}

func (ts *TargetServer) Config() dskit.Config {
	return *ts.config
}
