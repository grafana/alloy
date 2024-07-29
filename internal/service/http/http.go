// Package http implements the HTTP service.
package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/remotecfg"
	"github.com/grafana/alloy/internal/static/server"
	"github.com/grafana/ckit/memconn"
	_ "github.com/grafana/pyroscope-go/godeltaprof/http/pprof" // Register godeltaprof handler
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// ServiceName defines the name used for the HTTP service.
const ServiceName = "http"

// Options are used to configure the HTTP service. Options are constant for the
// lifetime of the HTTP service.
type Options struct {
	Logger   log.Logger           // Where to send logs.
	Tracer   trace.TracerProvider // Where to send traces.
	Gatherer prometheus.Gatherer  // Where to collect metrics from.

	ReadyFunc  func() bool
	ReloadFunc func() (*alloy_runtime.Source, error)

	HTTPListenAddr   string // Address to listen for HTTP traffic on.
	MemoryListenAddr string // Address to accept in-memory traffic on.
	EnablePProf      bool   // Whether pprof endpoints should be exposed.
}

// Arguments holds runtime settings for the HTTP service.
type Arguments struct {
	TLS *TLSArguments `alloy:"tls,block,optional"`
}

type Service struct {
	log      log.Logger
	tracer   trace.TracerProvider
	gatherer prometheus.Gatherer
	opts     Options

	winMut sync.Mutex
	win    *server.WinCertStoreHandler

	// publicLis and tcpLis are used to lazily enable TLS, since TLS is
	// optionally configurable at runtime.
	//
	// publicLis is the listener that is exposed to the public. It either sends
	// traffic directly to tcpLis, or sends it to an intermediate TLS listener
	// when TLS is enabled.
	//
	// tcpLis forwards traffic to a TCP listener once the Service is running; it
	// is lazily initiated since we don't listen to traffic until the Service
	// runs.
	publicLis, tcpLis *lazyListener

	memLis *memconn.Listener

	componentHttpPathPrefix          string
	componentHttpPathPrefixRemotecfg string
}

var _ service.Service = (*Service)(nil)

// New returns a new, unstarted instance of the HTTP service.
func New(opts Options) *Service {
	var (
		l = opts.Logger
		t = opts.Tracer
		r = opts.Gatherer
	)

	if l == nil {
		l = log.NewNopLogger()
	}
	if t == nil {
		t = noop.NewTracerProvider()
	}
	if r == nil {
		r = prometheus.NewRegistry()
	}

	var (
		tcpLis    = &lazyListener{}
		publicLis = &lazyListener{}
	)

	// lazyLis should default to wrapping around lazyNetLis.
	_ = publicLis.SetInner(tcpLis)

	return &Service{
		log:      l,
		tracer:   t,
		gatherer: r,
		opts:     opts,

		publicLis: publicLis,
		tcpLis:    tcpLis,
		memLis:    memconn.NewListener(l),

		componentHttpPathPrefix:          "/api/v0/component/",
		componentHttpPathPrefixRemotecfg: "/api/v0/component/remotecfg",
	}
}

// Definition returns the definition of the HTTP service.
func (s *Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: Arguments{},
		DependsOn:  nil, //[]string{remotecfg.ServiceName}, // http requires remotecfg to be up to wire lookups to its controller.
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

// Run starts the HTTP service. It will run until the provided context is
// canceled or there is a fatal error.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		s.winMut.Lock()
		defer s.winMut.Unlock()
		if s.win != nil {
			s.win.Stop()
		}
	}()

	netLis, err := net.Listen("tcp", s.opts.HTTPListenAddr)
	if err != nil {
		// There is no recovering from failing to listen on the port.
		level.Error(s.log).Log("msg", fmt.Sprintf("failed to listen on %s", s.opts.HTTPListenAddr), "err", err)
		os.Exit(1)
	}
	if err := s.tcpLis.SetInner(netLis); err != nil {
		return fmt.Errorf("failed to use listener: %w", err)
	}

	r := mux.NewRouter()
	r.Use(otelmux.Middleware(
		"alloy",
		otelmux.WithTracerProvider(s.tracer),
	))

	r.Handle(
		"/metrics",
		promhttp.HandlerFor(s.gatherer, promhttp.HandlerOpts{}),
	)
	if s.opts.EnablePProf {
		r.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)
	}

	svc, ok := host.GetService(remotecfg.ServiceName)
	if !ok {
		// This will never happen as the service dependency is explicit.
		return fmt.Errorf("failed to get the remotecfg service when setting up the http service")
	}
	remotecfgHost := svc.Data().(remotecfg.Data).Host

	// NOTE(@tpaschalis) These need to be kept in order for the longer
	// remotecfg prefix to be invoked correctly.
	r.PathPrefix(s.componentHttpPathPrefixRemotecfg).Handler(s.componentHandler(remotecfgHost, s.componentHttpPathPrefixRemotecfg))
	r.PathPrefix(s.componentHttpPathPrefix).Handler(s.componentHandler(host, s.componentHttpPathPrefix))

	if s.opts.ReadyFunc != nil {
		r.HandleFunc("/-/ready", func(w http.ResponseWriter, _ *http.Request) {
			if s.opts.ReadyFunc() {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, "Alloy is ready.")
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintln(w, "Alloy is not ready.")
			}
		})
	}

	if s.opts.ReloadFunc != nil {
		r.HandleFunc("/-/reload", func(w http.ResponseWriter, _ *http.Request) {
			level.Info(s.log).Log("msg", "reload requested via /-/reload endpoint")

			_, err := s.opts.ReloadFunc()
			if err != nil {
				level.Error(s.log).Log("msg", "failed to reload config", "err", err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			level.Info(s.log).Log("msg", "config reloaded")
			_, _ = fmt.Fprintln(w, "config reloaded")
		}).Methods(http.MethodGet, http.MethodPost)
	}

	// Wire custom service handlers for services which depend on the http
	// service.
	//
	// NOTE(rfratto): keep this at the bottom of all other routes, otherwise a
	// service with a colliding path takes precedence over a predefined route.
	for _, route := range s.getServiceRoutes(host) {
		r.PathPrefix(route.Base).Handler(route.Handler)
	}

	srv := &http.Server{Handler: h2c.NewHandler(r, &http2.Server{})}

	level.Info(s.log).Log("msg", "now listening for http traffic", "addr", s.opts.HTTPListenAddr)

	listeners := []net.Listener{s.publicLis, s.memLis}
	for _, lis := range listeners {
		wg.Add(1)
		go func(lis net.Listener) {
			defer wg.Done()
			defer cancel()

			if err := srv.Serve(lis); err != nil {
				level.Info(s.log).Log("msg", "http server closed", "addr", lis.Addr(), "err", err)
			}
		}(lis)
	}

	defer func() { _ = srv.Shutdown(ctx) }()

	<-ctx.Done()
	return nil
}

// getServiceRoutes returns a sorted list of service routes for services which
// depend on the HTTP service.
//
// Longer paths are prioritized over shorter paths so that a service with a
// more specific base route takes precedence.
func (s *Service) getServiceRoutes(host service.Host) []serviceRoute {
	var routes serviceRoutes

	for _, consumer := range host.GetServiceConsumers(ServiceName) {
		if consumer.Type != service.ConsumerTypeService {
			continue
		}

		sh, ok := consumer.Value.(ServiceHandler)
		if !ok {
			continue
		}
		base, handler := sh.ServiceHandler(host)

		routes = append(routes, serviceRoute{
			Base:    base,
			Handler: handler,
		})
	}

	sort.Sort(routes)
	return routes
}

func (s *Service) componentHandler(host service.Host, pathPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Trim the path prefix to get our full path.
		trimmedPath := strings.TrimPrefix(r.URL.Path, pathPrefix)

		// splitURLPath should only fail given an unexpected path.
		componentID, componentPath, err := splitURLPath(host, trimmedPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "failed to parse URL path %q: %s\n", r.URL.Path, err)
		}

		info, err := host.GetComponent(componentID, component.InfoOptions{})
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		component, ok := info.Component.(Component)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler := component.Handler()
		if handler == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Send just the remaining path to our component so each component can
		// handle paths from their own root path.
		r.URL.Path = componentPath
		handler.ServeHTTP(w, r)
	}
}

// Update implements [service.Service] and applies settings.
func (s *Service) Update(newConfig any) error {
	newArgs := newConfig.(Arguments)

	if newArgs.TLS != nil {
		var tlsConfig *tls.Config
		var err error
		if newArgs.TLS.WindowsFilter != nil {
			err = s.updateWindowsCertificateFilter(newArgs.TLS)
			if err != nil {
				return err
			}
			tlsConfig, err = newArgs.TLS.winTlsConfig(s.win)
		} else {
			tlsConfig, err = newArgs.TLS.tlsConfig()
		}
		if err != nil {
			return err
		}

		newTLSListener := tls.NewListener(s.tcpLis, tlsConfig)
		level.Info(s.log).Log("msg", "applying TLS config to HTTP server")
		if err := s.publicLis.SetInner(newTLSListener); err != nil {
			return err
		}
	} else {
		// Ensure that the outer lazy listener is sending requests directly to the
		// network, instead of any previous instance of a TLS listener.
		level.Info(s.log).Log("msg", "applying non-TLS config to HTTP server")
		if err := s.publicLis.SetInner(s.tcpLis); err != nil {
			return err
		}
	}

	return nil
}

// Data returns an instance of [Data]. Calls to Data are cachable by the
// caller.
//
// Data must only be called after parsing command-line flags.
func (s *Service) Data() any {
	return Data{
		HTTPListenAddr:   s.opts.HTTPListenAddr,
		MemoryListenAddr: s.opts.MemoryListenAddr,
		BaseHTTPPath:     s.componentHttpPathPrefix,

		DialFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
			switch address {
			case s.opts.MemoryListenAddr:
				return s.memLis.DialContext(ctx)
			default:
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}
		},
	}
}

// Data includes information associated with the HTTP service.
type Data struct {
	// Address that the HTTP service is configured to listen on.
	HTTPListenAddr string

	// Address that the HTTP service is configured to listen on for in-memory
	// traffic when [DialFunc] is used to establish a connection.
	MemoryListenAddr string

	// BaseHTTPPath is the base path where component HTTP routes are exposed.
	BaseHTTPPath string

	// DialFunc is a function which establishes in-memory network connection when
	// address is MemoryListenAddr. If address is not MemoryListenAddr, DialFunc
	// establishes an outbound network connection.
	DialFunc func(ctx context.Context, network, address string) (net.Conn, error)
}

// HTTPPathForComponent returns the full HTTP path for a given global component
// ID.
func (d Data) HTTPPathForComponent(componentID string) string {
	merged := path.Join(d.BaseHTTPPath, componentID)
	if !strings.HasSuffix(merged, "/") {
		return merged + "/"
	}
	return merged
}

// Component is a component which also contains a custom HTTP handler.
type Component interface {
	component.Component

	// Handler should return a valid HTTP handler for the component.
	// All requests to the component will have the path trimmed such that the component is at the root.
	// For example, f a request is made to `/component/{id}/metrics`, the component
	// will receive a request to just `/metrics`.
	Handler() http.Handler
}

// ServiceHandler is a Service which exposes custom HTTP handlers.
type ServiceHandler interface {
	service.Service

	// ServiceHandler returns the base route and HTTP handlers to register for
	// the provided service.
	//
	// This method is only called for services that declare a dependency on
	// the http service.
	//
	// The http service prioritizes longer base routes. Given two base routes of
	// /foo and /foo/bar, an HTTP URL of /foo/bar/baz will be routed to the
	// longer base route (/foo/bar).
	ServiceHandler(host service.Host) (base string, handler http.Handler)
}

// lazyListener is a [net.Listener] which lazily initializes the underlying
// listener.
type lazyListener struct {
	mut    sync.RWMutex
	inner  net.Listener
	closed bool
}

var _ net.Listener = (*lazyListener)(nil)

// SetInner updates the inner listener. It is safe to call SetInner multiple
// times. SetInner panics if given a nil argument.
//
// SetInner returns an error if called after the listener is closed.
func (lis *lazyListener) SetInner(inner net.Listener) error {
	if inner == nil {
		panic("Unexpected nil listener passed to SetInner")
	}

	lis.mut.Lock()
	defer lis.mut.Unlock()

	if lis.closed {
		return net.ErrClosed
	}

	lis.inner = inner
	return nil
}

func (lis *lazyListener) Accept() (net.Conn, error) {
	// The read lock is held as briefly as possible since Accept is a blocking
	// call and may hold the read lock longer than we want it to.
	lis.mut.RLock()
	var (
		inner  = lis.inner
		closed = lis.closed
	)
	lis.mut.RUnlock()

	if closed || inner == nil {
		return nil, net.ErrClosed
	}
	return inner.Accept()
}

func (lis *lazyListener) Close() error {
	lis.mut.Lock()
	defer lis.mut.Unlock()

	if lis.closed {
		return net.ErrClosed
	}

	lis.closed = true
	return lis.inner.Close()
}

func (lis *lazyListener) Addr() net.Addr {
	lis.mut.RLock()
	defer lis.mut.RUnlock()

	if lis.inner == nil {
		// TODO(rfratto): it's not sure if this will cause problems. If this is an
		// issue, we can do one of two things to address this:
		//
		// 1. Return a fake address.
		// 2. Block until lis.inner is set (using a sync.Cond) and then return the
		//    inner address.
		return nil
	}

	return lis.inner.Addr()
}
