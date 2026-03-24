package net

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	dskit "github.com/grafana/dskit/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"golang.org/x/net/http2/h2c"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// TargetServer is wrapper around dskit.Server that handles some common
// configuration used in all components that expose a network server. It just
// handles configuration and initialization, the handlers implementation are
// left to the consumer.
type TargetServer struct {
	logger           log.Logger
	config           *dskit.Config
	metricsNamespace string
	server           *dskit.Server
	http2            *HTTP2Config
}

// NewTargetServer creates a new TargetServer, applying some defaults to the server configuration.
// If provided config is nil, a default configuration will be used instead.
func NewTargetServer(logger log.Logger, metricsNamespace string, reg prometheus.Registerer, config *ServerConfig) (*TargetServer, error) {
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
	ts.config.Log = ts.logger

	return ts, nil
}

// MountAndRun mounts the handlers and starting the server.
func (ts *TargetServer) MountAndRun(mountRoute func(router *mux.Router)) error {
	level.Info(ts.logger).Log("msg", "starting server")
	srv, err := dskit.New(*ts.config)
	if err != nil {
		return err
	}

	ts.server = srv

	if http2Server := ts.http2.Server(); http2Server != nil {
		ts.server.HTTPServer.Handler = h2c.NewHandler(ts.server.HTTPServer.Handler, http2Server)
	}
	mountRoute(ts.server.HTTP)

	go func() {
		err := srv.Run()
		if err != nil {
			level.Error(ts.logger).Log("msg", "server shutdown with error", "err", err)
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
