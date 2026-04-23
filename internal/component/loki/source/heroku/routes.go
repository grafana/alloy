package heroku

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	herokuEncoding "github.com/heroku/x/logplex/encoding"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source"
)

const (
	pathDrain   = "/heroku/api/v1/drain"
	pathHealthy = "/healthy"

	reservedLabelTenantID = "__tenant_id__"
)

func newRoutes(logger log.Logger, metrics *metrics) []source.LogsRoute {
	return []source.LogsRoute{
		newDrainRoute(logger, metrics),
	}
}

type drainRoute struct {
	metrics *metrics
}

func newDrainRoute(logger log.Logger, metrics *metrics) *drainRoute {
	_ = logger
	return &drainRoute{
		metrics: metrics,
	}
}

func (d *drainRoute) Path() string {
	return pathDrain
}

func (d *drainRoute) Method() string {
	return http.MethodPost
}

func (d *drainRoute) Logs(r *http.Request, cfg *source.LogsConfig) ([]loki.Entry, int, error) {
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

		if cfg.UseIncomingTimestamp {
			ts = message.Timestamp
		}

		for k, v := range r.URL.Query() {
			lb.Set(fmt.Sprintf("__heroku_drain_param_%s", k), strings.Join(v, ","))
		}

		tenantID := r.Header.Get("X-Scope-OrgID")
		if tenantID != "" {
			lb.Set(reservedLabelTenantID, tenantID)
		}

		processed, _ := relabel.Process(lb.Labels(), cfg.RelabelRules...)

		filtered := cfg.FixedLabels.Clone()
		processed.Range(func(lbl labels.Label) {
			if strings.HasPrefix(lbl.Name, "__") {
				return
			}
			filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
		})

		if tenantID != "" {
			filtered[reservedLabelTenantID] = model.LabelValue(tenantID)
		}

		entries = append(entries, loki.NewEntryWithCreated(filtered, created, push.Entry{
			Timestamp: ts,
			Line:      message.Message,
		}))
	}

	if err := herokuScanner.Err(); err != nil {
		d.metrics.parsingErrors.Inc()
		return nil, http.StatusBadRequest, err
	}

	return entries, http.StatusNoContent, nil
}

type healthyRoute struct{}

func newHealthyHandler() *healthyRoute {
	return &healthyRoute{}
}

func (h *healthyRoute) Path() string {
	return pathHealthy
}

func (h *healthyRoute) Method() string {
	return http.MethodGet
}

func (h *healthyRoute) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func ready(addr string) bool {
	resp, err := http.Get(fmt.Sprintf("http://%s%s", addr, pathHealthy))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
