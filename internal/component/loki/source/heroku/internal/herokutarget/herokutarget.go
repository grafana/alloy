package herokutarget

// This code is copied from Promtail. The herokutarget package is used to
// configure and run the targets that can read heroku entries and forward them
// to other loki components.

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	herokuEncoding "github.com/heroku/x/logplex/encoding"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/pkg/push"
)

const ReservedLabelTenantID = "__tenant_id__"

// HerokuDrainTargetConfig describes a scrape config to listen and consume heroku logs, in the HTTPS drain manner.
type HerokuDrainTargetConfig struct {
	Server *fnet.ServerConfig

	// Labels optionally holds labels to associate with each record received on the push api.
	Labels model.LabelSet

	// UseIncomingTimestamp sets the timestamp to the incoming heroku log entry timestamp. If false,
	// promtail will assign the current timestamp to the log entry when it was processed.
	UseIncomingTimestamp bool
}

type HerokuTarget struct {
	logger  log.Logger
	handler loki.LogsBatchReceiver

	once          sync.Once
	forceShutdown chan struct{}

	config         *HerokuDrainTargetConfig
	metrics        *Metrics
	relabelConfigs []*relabel.Config
	server         *fnet.TargetServer
}

// NewHerokuTarget creates a brand new Heroku Drain target, capable of receiving logs from a Heroku application through an HTTP drain.
func NewHerokuTarget(
	metrics *Metrics,
	logger log.Logger,
	handler loki.LogsBatchReceiver,
	relabel []*relabel.Config,
	config *HerokuDrainTargetConfig,
	reg prometheus.Registerer,
) (*HerokuTarget, error) {

	wrappedLogger := log.With(logger, "component", "heroku_drain")

	srv, err := fnet.NewTargetServer(wrappedLogger, "loki_source_heroku_drain_target", reg, config.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to create loki server: %w", err)
	}

	return &HerokuTarget{
		server:         srv,
		metrics:        metrics,
		logger:         wrappedLogger,
		handler:        handler,
		forceShutdown:  make(chan struct{}),
		config:         config,
		relabelConfigs: relabel,
	}, nil
}

func (h *HerokuTarget) Run() error {
	return h.server.MountAndRun(func(router *mux.Router) {
		router.Path(h.DrainEndpoint()).Methods("POST").Handler(http.HandlerFunc(h.drain))
		router.Path(h.HealthyEndpoint()).Methods("GET").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	})
}

func (h *HerokuTarget) drain(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	herokuScanner := herokuEncoding.NewDrainScanner(r.Body)

	var (
		entries []loki.Entry
		created = time.Now()
	)

	for herokuScanner.Scan() {
		ts := time.Now()
		message := herokuScanner.Message()
		lb := labels.NewBuilder(labels.EmptyLabels())
		lb.Set("__heroku_drain_host", message.Hostname)
		lb.Set("__heroku_drain_app", message.Application)
		lb.Set("__heroku_drain_proc", message.Process)
		lb.Set("__heroku_drain_log_id", message.ID)

		if h.config.UseIncomingTimestamp {
			ts = message.Timestamp
		}

		// Create __heroku_drain_param_<name> labels from query parameters
		params := r.URL.Query()
		for k, v := range params {
			lb.Set(fmt.Sprintf("__heroku_drain_param_%s", k), strings.Join(v, ","))
		}

		tenantIDHeaderValue := r.Header.Get("X-Scope-OrgID")
		if tenantIDHeaderValue != "" {
			// If present, first inject the tenant ID in, so it can be relabeled if necessary
			lb.Set(ReservedLabelTenantID, tenantIDHeaderValue)
		}

		processed, _ := relabel.Process(lb.Labels(), h.relabelConfigs...)

		// Start with the set of labels fixed in the configuration
		filtered := h.Labels().Clone()
		processed.Range(func(lbl labels.Label) {
			if strings.HasPrefix(lbl.Name, "__") {
				return
			}
			filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
		})

		// Then, inject it as the reserved label, so it's used by the remote write client
		if tenantIDHeaderValue != "" {
			filtered[ReservedLabelTenantID] = model.LabelValue(tenantIDHeaderValue)
		}

		entries = append(entries, loki.NewEntryWithCreated(filtered, created, push.Entry{
			Timestamp: ts,
			Line:      message.Message,
		}))
	}

	if err := herokuScanner.Err(); err != nil {
		h.metrics.herokuErrors.Inc()
		level.Warn(h.logger).Log("msg", "failed to read incoming heroku request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	numEntries := len(entries)
	if numEntries > 0 {
		select {
		case h.handler.Chan() <- entries:
		case <-r.Context().Done():
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		case <-h.forceShutdown:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		h.metrics.herokuEntries.Add(float64(numEntries))
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *HerokuTarget) Labels() model.LabelSet {
	return h.config.Labels
}

func (h *HerokuTarget) HTTPListenAddress() string {
	return h.server.HTTPListenAddr()
}

func (h *HerokuTarget) DrainEndpoint() string {
	return "/heroku/api/v1/drain"
}

func (h *HerokuTarget) HealthyEndpoint() string {
	return "/healthy"
}

func (h *HerokuTarget) Ready() bool {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", h.HTTPListenAddress(), h.HealthyEndpoint()), nil)
	if err != nil {
		return false
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		return false
	}

	return true
}

func (h *HerokuTarget) Details() any {
	return map[string]string{}
}

func (h *HerokuTarget) Shutdown() {
	level.Info(h.logger).Log("msg", "stopping heroku server")
	// StopAndShutdown tries to gracefully shutdown.
	// It will stop idle and incoming connections
	// and try to wait for all in-flight connections
	// to finish. If configured timeout `ServerGracefulShutdownTimeout`
	// expired this call will be unblocked.
	h.server.StopAndShutdown()

	// After we have tried a graceful shutdown we force all remaining in-flight
	// requests to exit.
	h.once.Do(func() { close(h.forceShutdown) })
}

// ForceShutdown will cancel all in-flight before starting server shutdown.
func (h *HerokuTarget) ForceShutdown() {
	level.Info(h.logger).Log("msg", "force shutdown of heroku server")
	h.once.Do(func() { close(h.forceShutdown) })
	h.server.StopAndShutdown()
}
