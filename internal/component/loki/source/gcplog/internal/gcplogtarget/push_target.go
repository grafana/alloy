package gcplogtarget

// This code is copied from Promtail. The gcplogtarget package is used to
// configure and run the targets that can read log entries from cloud resource
// logs like bucket logs, load balancer logs, and Kubernetes cluster logs
// from GCP.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/loki/source/gcplog/gcptypes"
)

// PushTarget defines a server for receiving messages from a GCP PubSub push
// subscription.
type PushTarget struct {
	logger  log.Logger
	metrics *Metrics
	recv    loki.LogsReceiver

	once          sync.Once
	forceShutdown chan struct{}

	server         *fnet.TargetServer
	config         *gcptypes.PushConfig
	relabelConfigs []*relabel.Config
}

// NewPushTarget constructs a PushTarget.
func NewPushTarget(
	metrics *Metrics,
	logger log.Logger,
	recv loki.LogsReceiver,
	config *gcptypes.PushConfig,
	relabel []*relabel.Config,
	reg prometheus.Registerer,
) (*PushTarget, error) {

	logger = log.With(logger, "component", "gcp_push")
	srv, err := fnet.NewTargetServer(logger, "loki_source_gcplog_push", reg, config.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcp push server: %w", err)
	}

	return &PushTarget{
		logger:         logger,
		metrics:        metrics,
		recv:           recv,
		forceShutdown:  make(chan struct{}),
		server:         srv,
		config:         config,
		relabelConfigs: relabel,
	}, nil
}

func (p *PushTarget) Run() error {
	return p.server.MountAndRun(func(router *mux.Router) {
		router.Path("/gcp/api/v1/push").Methods("POST").Handler(http.HandlerFunc(p.push))
	})
}

func (p *PushTarget) push(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Create no-op context.WithTimeout returns to simplify logic
	ctx := r.Context()
	if p.config.PushTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(r.Context(), p.config.PushTimeout)
		defer cancel()
	}

	pushMessage := pushMessageBody{}

	if err := json.NewDecoder(r.Body).Decode(&pushMessage); err != nil {
		p.metrics.gcpPushErrors.WithLabelValues("read_error").Inc()
		level.Warn(p.logger).Log("msg", "failed to read incoming gcp push request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := pushMessage.Validate(); err != nil {
		p.metrics.gcpPushErrors.WithLabelValues("invalid_message").Inc()
		level.Warn(p.logger).Log("msg", "invalid gcp push request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entry, err := parsePushMessage(pushMessage, p.relabelConfigs, r.Header.Get("X-Scope-OrgID"), parseOptions{
		fixedLabels:          p.labels(),
		useFullLine:          p.config.UseFullLine,
		useIncomingTimestamp: p.config.UseIncomingTimestamp,
	})

	if err != nil {
		p.metrics.gcpPushErrors.WithLabelValues("translation").Inc()
		level.Warn(p.logger).Log("msg", "failed to translate gcp push request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	select {
	case <-ctx.Done():
		// Request timeout from client or by configured timeout.
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	case <-p.forceShutdown:
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	case p.recv.Chan() <- entry:
		p.metrics.gcpPushEntries.WithLabelValues().Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}
}

// labels return the model.LabelSet that the target applies to log entries.
func (p *PushTarget) labels() model.LabelSet {
	lbls := make(model.LabelSet, len(p.config.Labels))
	for k, v := range p.config.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}
	return lbls
}

// Details returns some debug information about the target.
func (p *PushTarget) Details() map[string]string {
	return map[string]string{
		"strategy":       "push",
		"labels":         p.labels().String(),
		"server_address": p.server.HTTPListenAddr(),
	}
}

// Stop shuts down the push server.
func (p *PushTarget) Stop() {
	level.Info(p.logger).Log("msg", "stopping gcp push server")
	// StopAndShutdown tries to gracefully shutdown.
	// It will stop idle and incoming connections
	// and try to wait for all in-flight connections
	// to finish. If configured timeout `ServerGracefulShutdownTimeout`
	// expired this call will be unblocked.
	p.server.StopAndShutdown()

	// After we have tried a graceful shutdown we force all remaining in-flight
	// requests to exit.
	p.once.Do(func() { close(p.forceShutdown) })
}
