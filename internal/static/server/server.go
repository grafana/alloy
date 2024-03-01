// Package server implements the HTTP and gRPC server used throughout Grafana
// Agent.
//
// It is a grafana/agent-specific fork of github.com/weaveworks/common/server.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // anonymous import to get the pprof handler registered
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/ckit/memconn"
	"github.com/grafana/dskit/middleware"
	_ "github.com/grafana/pyroscope-go/godeltaprof/http/pprof" // anonymous import to get the godeltaprof handler registered
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/hashicorp/go-multierror"
	"github.com/oklog/run"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/netutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// DialContextFunc is a function matching the signature of
// net.Dialer.DialContext.
type DialContextFunc func(ctx context.Context, network string, addr string) (net.Conn, error)

// Server wraps an HTTP and gRPC server with some common initialization.
//
// Unless instrumentation is disabled in the Servers config, Prometheus metrics
// will be automatically generated for the server.
type Server struct {
	flagsMut sync.Mutex
	flags    Flags

	// Listeners for in-memory connections. These never use TLS.
	httpMemListener *memconn.Listener
	grpcMemListener *memconn.Listener

	// Listeners to use for connections. These will use TLS when TLS is enabled.
	httpListener net.Listener
	grpcListener net.Listener

	updateHTTPTLS func(TLSConfig) error
	updateGRPCTLS func(TLSConfig) error

	HTTP       *mux.Router
	HTTPServer *http.Server
	GRPC       *grpc.Server

	// DialContext creates a connection to the given network/address. If address
	// matches the Server's internal HTTP or gRPC address, an internal in-memory
	// connection will be opened.
	DialContext DialContextFunc
}

type metrics struct {
	tcpConnections      *prometheus.GaugeVec
	tcpConnectionsLimit *prometheus.GaugeVec
	requestDuration     *prometheus.HistogramVec
	receivedMessageSize *prometheus.HistogramVec
	sentMessageSize     *prometheus.HistogramVec
	inflightRequests    *prometheus.GaugeVec
}

func newMetrics(r prometheus.Registerer) (*metrics, error) {
	var m metrics

	// Create metrics for the server
	m.tcpConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "agent_tcp_connections",
		Help: "Current number of accepted TCP connections.",
	}, []string{"protocol"})
	m.tcpConnectionsLimit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "agent_tcp_connections_limit",
		Help: "The maximum number of TCP connections that can be accepted (0 = unlimited)",
	}, []string{"protocol"})
	m.requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "agent_request_duration_seconds",
		Help: "Time in seconds spent serving HTTP requests.",
	}, []string{"method", "route", "status_code", "ws"})
	m.receivedMessageSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agent_request_message_bytes",
		Help:    "Size (in bytes) of messages received in the request.",
		Buckets: middleware.BodySizeBuckets,
	}, []string{"method", "route"})
	m.sentMessageSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agent_response_message_bytes",
		Help:    "Size (in bytes) of messages sent in response.",
		Buckets: middleware.BodySizeBuckets,
	}, []string{"method", "route"})
	m.inflightRequests = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "agent_inflight_requests",
		Help: "Current number of inflight requests.",
	}, []string{"method", "route"})

	if r != nil {
		// Register all of our metrics
		cc := []prometheus.Collector{
			m.tcpConnections, m.tcpConnectionsLimit, m.requestDuration, m.receivedMessageSize,
			m.sentMessageSize, m.inflightRequests,
		}
		for _, c := range cc {
			if err := r.Register(c); err != nil {
				return nil, fmt.Errorf("failed registering server metrics: %w", err)
			}
		}
	}
	return &m, nil
}

// New creates a new Server with the given config.
//
// r is used to register Server-specific metrics. If r is nil, no metrics will
// be registered.
//
// g is used for collecting metrics from the instrumentation handlers, when
// enabled. If g is nil, a /metrics endpoint will not be registered.
func New(l log.Logger, r prometheus.Registerer, g prometheus.Gatherer, cfg Config, flags Flags) (srv *Server, err error) {
	if l == nil {
		l = log.NewNopLogger()
	}

	switch {
	case flags.HTTP.InMemoryAddr == "":
		return nil, fmt.Errorf("in memory HTTP address must be configured")
	case flags.GRPC.InMemoryAddr == "":
		return nil, fmt.Errorf("in memory gRPC address must be configured")
	case flags.HTTP.InMemoryAddr == flags.GRPC.InMemoryAddr:
		return nil, fmt.Errorf("in memory HTTP and gRPC address must be different")
	}

	m, err := newMetrics(r)
	if err != nil {
		return nil, err
	}

	// Create listeners first so we can fail early if the port is in use.
	httpListener, err := newHTTPListener(&flags.HTTP, m)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = httpListener.Close()
		}
	}()
	grpcListener, err := newGRPCListener(&flags.GRPC, m)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = httpListener.Close()
		}
	}()

	// Configure TLS
	var (
		updateHTTPTLS func(TLSConfig) error
		updateGRPCTLS func(TLSConfig) error
	)
	if flags.HTTP.UseTLS {
		httpTLSListener, err := newTLSListener(httpListener, cfg.HTTP.TLSConfig, l)
		if err != nil {
			return nil, fmt.Errorf("generating HTTP TLS config: %w", err)
		}
		httpListener = httpTLSListener
		updateHTTPTLS = httpTLSListener.ApplyConfig
	}
	if flags.GRPC.UseTLS {
		grpcTLSListener, err := newTLSListener(grpcListener, cfg.GRPC.TLSConfig, l)
		if err != nil {
			return nil, fmt.Errorf("generating GRPC TLS config: %w", err)
		}
		grpcListener = grpcTLSListener
		updateGRPCTLS = grpcTLSListener.ApplyConfig
	}

	level.Info(l).Log(
		"msg", "server listening on addresses",
		"http", httpListener.Addr(), "grpc", grpcListener.Addr(),
		"http_tls_enabled", flags.HTTP.UseTLS, "grpc_tls_enabled", flags.GRPC.UseTLS,
	)

	// Build servers
	grpcServer := newGRPCServer(l, &flags.GRPC, m)
	httpServer, router, err := newHTTPServer(l, g, &flags, m)
	if err != nil {
		return nil, err
	}

	// Build in-memory listeners and dial function
	var (
		httpMemListener = memconn.NewListener(nil)
		grpcMemListener = memconn.NewListener(nil)
	)
	dialFunc := func(ctx context.Context, network string, address string) (net.Conn, error) {
		switch address {
		case flags.HTTP.InMemoryAddr:
			return httpMemListener.DialContext(ctx)
		case flags.GRPC.InMemoryAddr:
			return grpcMemListener.DialContext(ctx)
		default:
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}
	}

	return &Server{
		flags:           flags,
		httpListener:    httpListener,
		grpcListener:    grpcListener,
		httpMemListener: httpMemListener,
		grpcMemListener: grpcMemListener,

		updateHTTPTLS: updateHTTPTLS,
		updateGRPCTLS: updateGRPCTLS,

		HTTP:        router,
		HTTPServer:  httpServer,
		GRPC:        grpcServer,
		DialContext: dialFunc,
	}, nil
}

func newHTTPListener(opts *HTTPFlags, m *metrics) (net.Listener, error) {
	httpAddress := opts.ListenAddress
	if httpAddress == "" {
		return nil, fmt.Errorf("http address not set")
	}
	httpListener, err := net.Listen(opts.ListenNetwork, httpAddress)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP listener: %w", err)
	}
	httpListener = middleware.CountingListener(httpListener, m.tcpConnections.WithLabelValues("http"))

	m.tcpConnectionsLimit.WithLabelValues("http").Set(float64(opts.ConnLimit))
	if opts.ConnLimit > 0 {
		httpListener = netutil.LimitListener(httpListener, opts.ConnLimit)
	}
	return httpListener, nil
}

func newGRPCListener(opts *GRPCFlags, m *metrics) (net.Listener, error) {
	grpcAddress := opts.ListenAddress
	if grpcAddress == "" {
		return nil, fmt.Errorf("gRPC address not set")
	}
	grpcListener, err := net.Listen(opts.ListenNetwork, grpcAddress)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC listener: %w", err)
	}
	grpcListener = middleware.CountingListener(grpcListener, m.tcpConnections.WithLabelValues("grpc"))

	m.tcpConnectionsLimit.WithLabelValues("grpc").Set(float64(opts.ConnLimit))
	if opts.ConnLimit > 0 {
		grpcListener = netutil.LimitListener(grpcListener, opts.ConnLimit)
	}
	return grpcListener, nil
}

func newGRPCServer(l log.Logger, opts *GRPCFlags, m *metrics) *grpc.Server {
	serverLog := middleware.GRPCServerLog{
		WithRequest: true,
		Log:         l,
	}
	grpcOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			serverLog.UnaryServerInterceptor,
			otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer()),
			middleware.UnaryServerInstrumentInterceptor(m.requestDuration),
		)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			serverLog.StreamServerInterceptor,
			otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer()),
			middleware.StreamServerInstrumentInterceptor(m.requestDuration),
		)),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     opts.MaxConnectionIdle,
			MaxConnectionAge:      opts.MaxConnectionAge,
			MaxConnectionAgeGrace: opts.MaxConnectionAgeGrace,
			Time:                  opts.KeepaliveTime,
			Timeout:               opts.KeepaliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             opts.MinTimeBetweenPings,
			PermitWithoutStream: opts.PingWithoutStreamAllowed,
		}),
		grpc.MaxRecvMsgSize(opts.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(opts.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(uint32(opts.MaxConcurrentStreams)),
		grpc.StatsHandler(middleware.NewStatsHandler(m.receivedMessageSize, m.sentMessageSize, m.inflightRequests)),
	}

	return grpc.NewServer(grpcOptions...)
}

func newHTTPServer(l log.Logger, g prometheus.Gatherer, opts *Flags, m *metrics) (*http.Server, *mux.Router, error) {
	router := mux.NewRouter()
	if opts.RegisterInstrumentation && g != nil {
		router.Handle("/metrics", promhttp.HandlerFor(g, promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		}))
		router.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)
	}

	var sourceIPs *middleware.SourceIPExtractor
	if opts.LogSourceIPs {
		var err error
		sourceIPs, err = middleware.NewSourceIPs(opts.LogSourceIPsHeader, opts.LogSourceIPsRegex)
		if err != nil {
			return nil, nil, fmt.Errorf("error setting up source IP extraction: %v", err)
		}
	}

	httpMiddleware := []middleware.Interface{
		middleware.Tracer{
			RouteMatcher: router,
			SourceIPs:    sourceIPs,
		},
		middleware.Log{
			Log:       l,
			SourceIPs: sourceIPs,
		},
		middleware.Instrument{
			RouteMatcher:     router,
			Duration:         m.requestDuration,
			RequestBodySize:  m.receivedMessageSize,
			ResponseBodySize: m.sentMessageSize,
			InflightRequests: m.inflightRequests,
		},
	}

	httpServer := &http.Server{
		ReadTimeout:  opts.HTTP.ReadTimeout,
		WriteTimeout: opts.HTTP.WriteTimeout,
		IdleTimeout:  opts.HTTP.IdleTimeout,
		Handler:      middleware.Merge(httpMiddleware...).Wrap(router),
	}

	return httpServer, router, nil
}

// HTTPAddress returns the HTTP net.Addr of this Server.
func (s *Server) HTTPAddress() net.Addr { return s.httpListener.Addr() }

// GRPCAddress returns the GRPC net.Addr of this Server.
func (s *Server) GRPCAddress() net.Addr { return s.grpcListener.Addr() }

// ApplyConfig applies changes to the Server block.
func (s *Server) ApplyConfig(cfg Config) error {
	s.flagsMut.Lock()
	defer s.flagsMut.Unlock()

	// N.B. LogLevel/LogFormat support dynamic updating but are never used in
	// *Server, so they're ignored here.

	if s.updateHTTPTLS != nil {
		if err := s.updateHTTPTLS(cfg.HTTP.TLSConfig); err != nil {
			return fmt.Errorf("updating HTTP TLS settings: %w", err)
		}
	}
	if s.updateGRPCTLS != nil {
		if err := s.updateGRPCTLS(cfg.GRPC.TLSConfig); err != nil {
			return fmt.Errorf("updating gRPC TLS settings: %w", err)
		}
	}

	return nil
}

// Run the server until en error is received or the given context is canceled.
// Run may not be re-called after it exits.
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var g run.Group

	g.Add(func() error {
		<-ctx.Done()
		return nil
	}, func(_ error) {
		cancel()
	})

	httpListeners := []net.Listener{
		s.httpListener,
		s.httpMemListener,
	}
	for i := range httpListeners {
		listener := httpListeners[i]
		g.Add(func() error {
			err := s.HTTPServer.Serve(listener)
			if errors.Is(err, http.ErrServerClosed) {
				err = nil
			}
			return err
		}, func(_ error) {
			ctx, cancel := context.WithTimeout(context.Background(), s.flags.GracefulShutdownTimeout)
			defer cancel()
			_ = s.HTTPServer.Shutdown(ctx)
		})
	}

	grpcListeners := []net.Listener{
		s.grpcListener,
		s.grpcMemListener,
	}
	for i := range grpcListeners {
		listener := grpcListeners[i]
		g.Add(func() error {
			err := s.GRPC.Serve(listener)
			if errors.Is(err, grpc.ErrServerStopped) {
				err = nil
			}
			return err
		}, func(_ error) {
			s.GRPC.GracefulStop()
		})
	}

	return g.Run()
}

// Close forcibly closes the server's listeners.
func (s *Server) Close() error {
	errs := multierror.Append(
		s.httpListener.Close(),
		s.grpcListener.Close(),
	)
	return errs.ErrorOrNil()
}
