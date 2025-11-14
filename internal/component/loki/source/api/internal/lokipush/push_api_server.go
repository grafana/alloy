package lokipush

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/tenant"
	"github.com/grafana/dskit/user"
	lokipush "github.com/grafana/loki/pkg/push"
	"github.com/grafana/loki/v3/pkg/loghttp/push"
	"github.com/grafana/loki/v3/pkg/util/constants"
	util_log "github.com/grafana/loki/v3/pkg/util/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	promql_parser "github.com/prometheus/prometheus/promql/parser"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	frelabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type PushAPIServer struct {
	logger       log.Logger
	serverConfig *fnet.ServerConfig
	server       *fnet.TargetServer
	handler      loki.LogsBatchReceiver

	once          sync.Once
	forceShutdown chan struct{}

	rwMutex            sync.RWMutex
	labels             model.LabelSet
	relabelRules       []*relabel.Config
	keepTimestamp      bool
	maxSendMessageSize int64
}

func NewPushAPIServer(logger log.Logger,
	serverConfig *fnet.ServerConfig,
	handler loki.LogsBatchReceiver,
	registerer prometheus.Registerer,
	maxSendMessageSize int64,
) (*PushAPIServer, error) {

	// Zero means default. This is done to match Loki's pushtarget.go behaviour.
	if maxSendMessageSize <= 0 {
		maxSendMessageSize = 100 << 20
	}

	s := &PushAPIServer{
		logger:             logger,
		serverConfig:       serverConfig,
		handler:            handler,
		forceShutdown:      make(chan struct{}),
		maxSendMessageSize: maxSendMessageSize,
	}

	srv, err := fnet.NewTargetServer(logger, "loki_source_api", registerer, serverConfig)
	if err != nil {
		return nil, err
	}

	s.server = srv
	return s, nil
}

func (s *PushAPIServer) Run() error {
	level.Info(s.logger).Log("msg", "starting push API server")

	err := s.server.MountAndRun(func(router *mux.Router) {
		// Extract the tenant ID from the request and add it to the context.
		tenantHeaderExtractor := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, ctx, _ := user.ExtractOrgIDFromHTTPRequest(r)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}

		// This redirecting is so we can avoid breaking changes where we originally implemented it with
		// the loki prefix.
		router.Path("/api/v1/push").Methods("POST").Handler(
			tenantHeaderExtractor(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.URL.Path = "/loki/api/v1/push"
					r.RequestURI = "/loki/api/v1/push"
					s.handleLoki(w, r)
				}),
			),
		)
		router.Path("/api/v1/raw").Methods("POST").Handler(
			tenantHeaderExtractor(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r.URL.Path = "/loki/api/v1/raw"
					r.RequestURI = "/loki/api/v1/raw"
					s.handlePlaintext(w, r)
				}),
			),
		)
		router.Path("/ready").Methods("GET").Handler(http.HandlerFunc(s.ready))
		router.Path("/loki/api/v1/push").Methods("POST").Handler(tenantHeaderExtractor(http.HandlerFunc(s.handleLoki)))
		router.Path("/loki/api/v1/raw").Methods("POST").Handler(tenantHeaderExtractor(http.HandlerFunc(s.handlePlaintext)))
	})
	return err
}

func (s *PushAPIServer) ServerConfig() fnet.ServerConfig {
	return *s.serverConfig
}

func (s *PushAPIServer) Shutdown() {
	level.Info(s.logger).Log("msg", "stopping push API server")
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

// ForceShutdown will cancel all in-flight before starting server shutdown.
func (s *PushAPIServer) ForceShutdown() {
	level.Info(s.logger).Log("msg", "force shutdown of push API server")
	s.once.Do(func() { close(s.forceShutdown) })
	s.server.StopAndShutdown()
}

func (s *PushAPIServer) SetLabels(labels model.LabelSet) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	s.labels = labels
}

func (s *PushAPIServer) getLabels() model.LabelSet {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()
	return s.labels.Clone()
}

func (s *PushAPIServer) SetKeepTimestamp(keepTimestamp bool) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	s.keepTimestamp = keepTimestamp
}

func (s *PushAPIServer) getKeepTimestamp() bool {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()
	return s.keepTimestamp
}

func (s *PushAPIServer) SetRelabelRules(rules frelabel.Rules) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	s.relabelRules = frelabel.ComponentToPromRelabelConfigs(rules)
}

func (s *PushAPIServer) getRelabelRules() []*relabel.Config {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()
	newRules := make([]*relabel.Config, len(s.relabelRules))
	for i, r := range s.relabelRules {
		var rCopy = *r
		newRules[i] = &rCopy
	}
	return newRules
}

// NOTE: This code is copied from Promtail (https://github.com/grafana/loki/commit/47e2c5884f443667e64764f3fc3948f8f11abbb8) with changes kept to the minimum.
// Only the HTTP handler functions are copied to allow for Alloy-specific server configuration and lifecycle management.
func (s *PushAPIServer) handleLoki(w http.ResponseWriter, r *http.Request) {
	logger := util_log.WithContext(r.Context(), util_log.Logger)
	tenantID, _ := tenant.TenantID(r.Context())
	req, _, err := push.ParseRequest(
		logger,
		tenantID,
		int(s.maxSendMessageSize),
		r,
		push.EmptyLimits{},
		nil,
		push.ParseLokiRequest,
		nil, // usage tracker
		nil,
		"",
		constants.Loki,
	)
	if err != nil {
		level.Warn(s.logger).Log("msg", "failed to parse incoming push request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Take snapshot of current configs and apply consistently for the entire request.
	addLabels := s.getLabels()
	relabelRules := s.getRelabelRules()
	keepTimestamp := s.getKeepTimestamp()

	var (
		entries []loki.Entry
		lastErr error
	)
	for _, stream := range req.Streams {
		ls, err := promql_parser.ParseMetric(stream.Labels)
		if err != nil {
			lastErr = err
			continue
		}

		lb := labels.NewBuilder(ls)

		// Add configured labels
		for k, v := range addLabels {
			lb.Set(string(k), string(v))
		}

		// Apply relabeling
		processed, keep := relabel.Process(lb.Labels(), relabelRules...)
		if !keep || processed.Len() == 0 {
			continue
		}

		// Convert to model.LabelSet
		filtered := model.LabelSet{}
		processed.Range(func(l labels.Label) {
			if strings.HasPrefix(l.Name, "__") {
				return
			}
			filtered[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		})

		// Add tenant ID to the filtered labels if it is set
		if tenantID != "" {
			filtered[model.LabelName(client.ReservedLabelTenantID)] = model.LabelValue(tenantID)
		}

		for _, entry := range stream.Entries {
			e := loki.Entry{
				Labels: filtered.Clone(),
				Entry: lokipush.Entry{
					Line:               entry.Line,
					StructuredMetadata: entry.StructuredMetadata,
					Parsed:             entry.Parsed,
				},
			}
			if keepTimestamp {
				e.Timestamp = entry.Timestamp
			} else {
				e.Timestamp = time.Now()
			}

			entries = append(entries, e)
		}
	}

	if len(entries) > 0 {
		select {
		case s.handler.Chan() <- entries:
		case <-r.Context().Done():
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		case <-s.forceShutdown:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		if lastErr != nil {
			level.Warn(s.logger).Log("msg", "at least one entry in the push request failed to process", "err", lastErr.Error())
			http.Error(w, lastErr.Error(), http.StatusBadRequest)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// NOTE: This code is copied from Promtail (https://github.com/grafana/loki/commit/47e2c5884f443667e64764f3fc3948f8f11abbb8) with changes kept to the minimum.
// Only the HTTP handler functions are copied to allow for Alloy-specific server configuration and lifecycle management.
func (s *PushAPIServer) handlePlaintext(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body := bufio.NewReader(r.Body)
	addLabels := s.getLabels()

	var entries []loki.Entry

	for {
		line, err := body.ReadString('\n')
		if err != nil && err != io.EOF {
			level.Warn(s.logger).Log("msg", "failed to read incoming push request", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if err == io.EOF {
				break
			}
			continue
		}

		entries = append(entries, loki.Entry{Labels: addLabels, Entry: lokipush.Entry{Timestamp: time.Now(), Line: line}})

		if err == io.EOF {
			break
		}
	}

	if len(entries) > 0 {
		select {
		case s.handler.Chan() <- entries:
		case <-r.Context().Done():
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		case <-s.forceShutdown:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// NOTE: This code is copied from Promtail (https://github.com/grafana/loki/commit/47e2c5884f443667e64764f3fc3948f8f11abbb8) with changes kept to the minimum.
// Only the HTTP handler functions are copied to allow for Alloy-specific server configuration and lifecycle management.
func (s *PushAPIServer) ready(w http.ResponseWriter, _ *http.Request) {
	resp := "ready"
	if _, err := w.Write([]byte(resp)); err != nil {
		level.Error(s.logger).Log("msg", "failed to respond to ready endpoint", "err", err)
	}
}
