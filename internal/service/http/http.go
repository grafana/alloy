// Package http implements the HTTP service.
package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/remotecfg"
	"github.com/grafana/alloy/internal/static/server"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/printer"
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
	Logger   *logging.Logger      // Where to send logs.
	Tracer   trace.TracerProvider // Where to send traces.
	Gatherer prometheus.Gatherer  // Where to collect metrics from.

	ReadyFunc  func() bool
	ReloadFunc func() error

	HTTPListenAddr   string                // Address to listen for HTTP traffic on.
	MemoryListenAddr string                // Address to accept in-memory traffic on.
	EnablePProf      bool                  // Whether pprof endpoints should be exposed.
	MinStability     featuregate.Stability // Minimum stability level to utilize for feature gates
	BundleContext    SupportBundleContext  // Context for delivering a support bundle
}

// Arguments holds runtime settings for the HTTP service.
type Arguments struct {
	Auth *AuthArguments `alloy:"auth,block,optional"`
	TLS  *TLSArguments  `alloy:"tls,block,optional"`
}

type Service struct {
	// globalLogger allows us to leverage the logging struct for setting a temporary
	// logger for support bundle usage and still leverage log.With for logging in the service
	globalLogger *logging.Logger
	log          log.Logger
	tracer       trace.TracerProvider
	gatherer     prometheus.Gatherer
	opts         Options

	winMut sync.Mutex
	win    *server.WinCertStoreHandler

	// Used to enforce single-flight requests to supportHandler
	supportBundleMut sync.Mutex

	// Track the raw config for use with the support bundle
	sources map[string]*ast.File

	authenticatorMut sync.RWMutex
	// authenticator is applied to every request made to http server
	authenticator authenticator

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
		l = logging.NewNop()
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
		globalLogger: l,
		log:          log.With(l, "service", "http"),
		tracer:       t,
		gatherer:     r,
		opts:         opts,

		authenticator: allowAuthenticator,

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
		DependsOn:  []string{remotecfg.ServiceName}, // http requires remotecfg to be up to wire lookups to its controller.
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

	// Apply authenticator middleware.
	// If none is configured allowAuthenticator is used and no authentication is required.
	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.authenticatorMut.RLock()
			err := s.authenticator(w, r)
			s.authenticatorMut.RUnlock()
			if err != nil {
				level.Info(s.log).Log("msg", "failed to authenticate request", "path", r.URL.Path, "err", err)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			h.ServeHTTP(w, r)
		})
	})

	// The implementation for "/-/healthy" is inspired by
	// the "/components" web API endpoint in /internal/web/api/api.go
	r.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
		components, err := host.ListComponents("", component.InfoOptions{
			GetHealth: true,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		unhealthyComponents := []string{}
		for _, c := range components {
			if c.Health.Health == component.HealthTypeUnhealthy {
				unhealthyComponents = append(unhealthyComponents, c.ComponentName)
			}
		}
		if len(unhealthyComponents) > 0 {
			http.Error(w, "unhealthy components: "+strings.Join(unhealthyComponents, ", "), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "All Alloy components are healthy.")
	})

	r.Handle(
		"/metrics",
		promhttp.HandlerFor(s.gatherer, promhttp.HandlerOpts{}),
	)
	if s.opts.EnablePProf {
		r.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)
	}

	// NOTE(@tpaschalis) These need to be kept in order for the longer
	// remotecfg prefix to be invoked correctly. The pathPrefix is still the
	// same so that `remotecfg/` is not stripped from component lookups.
	r.PathPrefix(s.componentHttpPathPrefixRemotecfg).Handler(s.componentHandler(remoteCfgHostProvider(host), s.componentHttpPathPrefix))
	r.PathPrefix(s.componentHttpPathPrefix).Handler(s.componentHandler(rootHostProvider(host), s.componentHttpPathPrefix))

	if s.opts.ReadyFunc != nil {
		r.HandleFunc("/-/ready", func(w http.ResponseWriter, _ *http.Request) {
			if s.opts.ReadyFunc() {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintln(w, "Alloy is ready.")
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = fmt.Fprintln(w, "Alloy is not ready.")
			}
		})
	}

	if s.opts.ReloadFunc != nil {
		r.HandleFunc("/-/reload", func(w http.ResponseWriter, _ *http.Request) {
			level.Info(s.log).Log("msg", "reload requested via /-/reload endpoint")

			if err := s.opts.ReloadFunc(); err != nil {
				level.Error(s.log).Log("msg", "failed to reload config", "err", err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			level.Info(s.log).Log("msg", "config reloaded")
			_, _ = fmt.Fprintln(w, "config reloaded")
		}).Methods(http.MethodGet, http.MethodPost)
	}

	// Wire in support bundle generator
	r.HandleFunc("/-/support", s.generateSupportBundleHandler(host)).Methods("GET")

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

// IsReady returns true if Alloy is deemed ready based on the provided ReadyFunc
func (s *Service) IsReady() bool {
	if s.opts.ReadyFunc == nil {
		return false
	}

	return s.opts.ReadyFunc()
}

func (s *Service) generateSupportBundleHandler(host service.Host) func(rw http.ResponseWriter, r *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		s.supportBundleMut.Lock()
		defer s.supportBundleMut.Unlock()

		if s.opts.BundleContext.DisableSupportBundle {
			rw.WriteHeader(http.StatusForbidden)
			_, _ = rw.Write([]byte("support bundle generation is disabled; it can be re-enabled by removing the --disable-support-bundle flag"))
			return
		}

		duration := getServerWriteTimeout(r)
		if r.URL.Query().Has("duration") {
			d, err := strconv.Atoi(r.URL.Query().Get("duration"))
			if err != nil {
				http.Error(rw, fmt.Sprintf("duration value (in seconds) should be a positive integer: %s", err), http.StatusBadRequest)
				return
			}
			if d < 1 {
				http.Error(rw, "duration value (in seconds) should be larger than 1", http.StatusBadRequest)
				return
			}
			if float64(d) > duration.Seconds() {
				http.Error(rw, "duration value exceeds the server's write timeout", http.StatusBadRequest)
				return
			}
			duration = time.Duration(d) * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()

		var logsBuffer bytes.Buffer
		syncBuff := log.NewSyncWriter(&logsBuffer)
		s.globalLogger.SetTemporaryWriter(syncBuff)
		defer func() {
			s.globalLogger.RemoveTemporaryWriter()
		}()

		// Get and redact the cached remote config.
		cachedConfig, err := remoteCfgRedactedCachedConfig(host)
		if err != nil {
			level.Debug(s.log).Log("msg", "failed to get cached remote config", "err", err)
		}

		// Ensure the sources are written using the printer as it will handle
		// secret redaction.
		sources := redactedSources(s.sources)

		bundle, err := ExportSupportBundle(ctx, s.opts.BundleContext.RuntimeFlags, s.opts.HTTPListenAddr, sources, cachedConfig, s.Data().(Data).DialFunc)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := ServeSupportBundle(rw, bundle, &logsBuffer); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// SetSources sets the sources on reload to be delivered
// with the support bundle.
func (s *Service) SetSources(sources map[string]*ast.File) {
	s.supportBundleMut.Lock()
	defer s.supportBundleMut.Unlock()
	s.sources = sources
}

func getServerWriteTimeout(r *http.Request) time.Duration {
	srv, ok := r.Context().Value(http.ServerContextKey).(*http.Server)
	if ok && srv.WriteTimeout != 0 {
		return srv.WriteTimeout
	}
	return 30 * time.Second
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

func (s *Service) componentHandler(getHost func() (service.Host, error), pathPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, err := getHost()
		if host == nil || err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "failed to get host: %s\n", err)
			return
		}
		// Trim the path prefix to get our full path.
		trimmedPath := strings.TrimPrefix(r.URL.Path, pathPrefix)

		// splitURLPath should only fail given an unexpected path.
		componentID, componentPath, err := splitURLPath(host, trimmedPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "failed to parse URL path %q: %s\n", r.URL.Path, err)
			return
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

	s.authenticatorMut.Lock()
	if newArgs.Auth != nil {
		s.authenticator = newArgs.Auth.authenticator()
	} else {
		s.authenticator = allowAuthenticator
	}
	s.authenticatorMut.Unlock()

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

func redactedSources(sources map[string]*ast.File) map[string][]byte {
	if sources == nil {
		return nil
	}
	printedSources := make(map[string][]byte, len(sources))

	for k, v := range sources {
		b, err := printFileRedacted(v)
		if err != nil {
			printedSources[k] = []byte(fmt.Errorf("failed to print source: %w", err).Error())
			continue
		}
		printedSources[k] = b
	}
	return printedSources
}

func remoteCfgRedactedCachedConfig(host service.Host) ([]byte, error) {
	svc, ok := host.GetService(remotecfg.ServiceName)
	if !ok {
		return nil, fmt.Errorf("failed to get the remotecfg service")
	}

	return printFileRedacted(svc.(*remotecfg.Service).GetCachedAstFile())
}

func printFileRedacted(f *ast.File) ([]byte, error) {
	if f == nil {
		return []byte{}, nil
	}

	c := printer.Config{
		RedactSecrets: true,
	}

	var buf bytes.Buffer
	w := io.Writer(&buf)
	if err := c.Fprint(w, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func remoteCfgHostProvider(host service.Host) func() (service.Host, error) {
	return func() (service.Host, error) {
		return remotecfg.GetHost(host)
	}
}

func rootHostProvider(host service.Host) func() (service.Host, error) {
	return func() (service.Host, error) {
		return host, nil
	}
}
