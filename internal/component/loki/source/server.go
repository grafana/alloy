package source

import (
	"net/http"
	"reflect"
	"sync"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
)

// Server exposes HTTP routes that ingest log entries and forward them in batches.
type Server struct {
	logger  log.Logger
	metrics *serverMetrics

	server    *fnet.TargetServer
	netConfig *fnet.ServerConfig

	mut        sync.RWMutex
	logsConfig *LogsConfig

	recv loki.LogsBatchReceiver

	once          sync.Once
	forceShutdown chan struct{}
}

type HTTPRoute interface {
	Path() string
	Method() string
}

// LogsRoute describes an HTTP endpoint that produces log entries.
type LogsRoute interface {
	HTTPRoute
	// Logs converts a request into log entries and an HTTP status code.
	// If it returns no entries and a non-nil error, the request is rejected.
	// If it returns entries, they are forwarded before the status code is written.
	// Returning both entries and an error reports partial success using the returned status.
	Logs(r *http.Request, opts *LogsConfig) ([]loki.Entry, int, error)
}

// HandlerRoute describes an HTTP endpoint handled directly with an http.Handler.
type HandlerRoute interface {
	HTTPRoute
	http.Handler
}

type ServerConfig struct {
	Namespace  string
	NetConfig  *fnet.ServerConfig
	LogsConfig *LogsConfig
}

type LogsConfig struct {
	FixedLabels          model.LabelSet
	RelabelRules         []*relabel.Config
	UseIncomingTimestamp bool
}

func NewServer(logger log.Logger, reg prometheus.Registerer, recv loki.LogsBatchReceiver, cfg ServerConfig) (*Server, error) {
	server, err := fnet.NewTargetServer(logger, cfg.Namespace, reg, cfg.NetConfig)
	if err != nil {
		return nil, err
	}

	return &Server{
		logger:        logger,
		metrics:       newServerMetrics(cfg.Namespace, reg),
		server:        server,
		netConfig:     cfg.NetConfig,
		logsConfig:    cfg.LogsConfig,
		recv:          recv,
		once:          sync.Once{},
		forceShutdown: make(chan struct{}),
	}, nil
}

// Run registers the configured routes and starts the server.
func (s *Server) Run(logs []LogsRoute, handlers []HandlerRoute) error {
	return s.server.MountAndRun(func(router *mux.Router) {
		for _, l := range logs {
			router.Path(l.Path()).Methods(l.Method()).Handler(s.logsHandler(l.Logs))
		}

		for _, h := range handlers {
			router.Path(h.Path()).Methods(h.Method()).Handler(h)
		}
	})
}

// Update replaces the configuration used for incoming log requests.
func (s *Server) Update(logsConfig *LogsConfig) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.logsConfig = logsConfig
}

// NeedsRestart reports whether a new server instance is required.
func (s *Server) NeedsRestart(netConfig *fnet.ServerConfig) bool {
	if s == nil {
		return true
	}

	return !reflect.DeepEqual(netConfig, s.netConfig)
}

// HTTPAddr returns the server HTTP listen address.
func (s *Server) HTTPAddr() string {
	return s.server.HTTPListenAddr()
}

// Shutdown stops the server.
func (s *Server) Shutdown() {
	level.Info(s.logger).Log("msg", "stopping server")
	// StopAndShutdown tries to gracefully shutdown.
	// It will stop idle and incoming connections
	// and try to wait for all in-flight connections
	// to finish. If configured timeout `ServerGracefulShutdownTimeout`
	// expired this call will be unblocked.
	s.server.StopAndShutdown()

	// After we have tried a graceful shutdown we force all remaining in-flight
	// requests to exit.
	s.once.Do(func() { close(s.forceShutdown) })
}

// ForceShutdown stops the server without waiting for in-flight requests.
func (s *Server) ForceShutdown() {
	level.Info(s.logger).Log("msg", "force shutdown of server")
	s.once.Do(func() { close(s.forceShutdown) })
	s.server.StopAndShutdown()
}

func (s *Server) logsHandler(logsFn func(r *http.Request, opts *LogsConfig) ([]loki.Entry, int, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mut.RLock()
		logsConfig := s.logsConfig
		s.mut.RUnlock()

		entries, status, err := logsFn(r, logsConfig)
		numEntries := len(entries)

		if err != nil && numEntries == 0 {
			level.Warn(s.logger).Log("msg", "failed to parse request", "err", err)
			http.Error(w, err.Error(), status)
			return
		}

		if numEntries > 0 {
			select {
			case s.recv.Chan() <- entries:
			case <-r.Context().Done():
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			case <-s.forceShutdown:
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

			s.metrics.entriesWritten.Add(float64(numEntries))

			if err != nil {
				level.Warn(s.logger).Log("msg", "at least one entry failed to be processed", "err", err)
				http.Error(w, err.Error(), status)
				return
			}
		}

		w.WriteHeader(status)
	})
}

type serverMetrics struct {
	entriesWritten prometheus.Counter
}

func newServerMetrics(namespace string, reg prometheus.Registerer) *serverMetrics {
	m := &serverMetrics{
		entriesWritten: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "entries_written",
			Help:      "Total number of entries written.",
		}),
	}

	m.entriesWritten = util.MustRegisterOrGet(reg, m.entriesWritten).(prometheus.Counter)

	return m
}
