package api

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/dskit/tenant"
	"github.com/grafana/dskit/user"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	promql_parser "github.com/prometheus/prometheus/promql/parser"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/api/internal/loghttp"
)

const (
	pathPush     = "/api/v1/push"
	pathLokiPush = "/loki/api/v1/push"

	pathRaw     = "/api/v1/raw"
	pathLokiRaw = "/loki/api/v1/raw"

	pathReady = "/ready"

	defaultMaxMessageSize = 100 << 20 // 100MiB
)

func newRoutes(maxMessageSize int) ([]source.LogsRoute, []source.HandlerRoute) {
	if maxMessageSize <= 0 {
		maxMessageSize = defaultMaxMessageSize
	}

	return []source.LogsRoute{
			newLokiRoute(pathPush, maxMessageSize),
			newLokiRoute(pathLokiPush, maxMessageSize),
			newPlainTextRoute(pathRaw),
			newPlainTextRoute(pathLokiRaw),
		}, []source.HandlerRoute{
			newReadyHandler(),
		}
}

var _ source.LogsRoute = (*lokiRoute)(nil)

func newLokiRoute(path string, maxMessageSize int) *lokiRoute {
	return &lokiRoute{
		path:           path,
		maxMessageSize: maxMessageSize,
	}
}

type lokiRoute struct {
	path           string
	maxMessageSize int
}

func (h *lokiRoute) Method() string {
	return http.MethodPost
}

func (h *lokiRoute) Path() string {
	return h.path
}

func (h *lokiRoute) Logs(r *http.Request, cfg *source.LogsConfig) ([]loki.Entry, int, error) {
	req, err := loghttp.ParsePushRequest(r, h.maxMessageSize)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	var (
		lastErr error
		entries []loki.Entry

		created  = time.Now()
		tenantID = getTenantID(r)
	)
	for _, stream := range req.Streams {
		ls, err := promql_parser.ParseMetric(stream.Labels)
		if err != nil {
			lastErr = err
			continue
		}

		lb := labels.NewBuilder(ls)

		// Add configured labels
		for k, v := range cfg.FixedLabels {
			lb.Set(string(k), string(v))
		}

		// Apply relabeling
		processed, keep := relabel.Process(lb.Labels(), cfg.RelabelRules...)
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
			// TODO(kalleep): pretty sure we don't have to clone here.
			e := loki.NewEntryWithCreated(filtered.Clone(), created, entry)
			if cfg.UseIncomingTimestamp {
				e.Timestamp = entry.Timestamp
			} else {
				e.Timestamp = time.Now()
			}

			entries = append(entries, e)
		}
	}

	if lastErr != nil {
		return entries, http.StatusBadRequest, lastErr
	}

	return entries, http.StatusNoContent, lastErr
}

var _ source.LogsRoute = (*plainTextRoute)(nil)

func newPlainTextRoute(path string) *plainTextRoute {
	return &plainTextRoute{path}
}

type plainTextRoute struct {
	path string
}

func (p *plainTextRoute) Path() string {
	return p.path
}

func (p *plainTextRoute) Method() string {
	return http.MethodPost
}

func (p *plainTextRoute) Logs(r *http.Request, cfg *source.LogsConfig) ([]loki.Entry, int, error) {
	defer r.Body.Close()
	body := bufio.NewReader(r.Body)

	var (
		entries []loki.Entry
		created = time.Now()
	)

	for {
		line, err := body.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, http.StatusBadRequest, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if err == io.EOF {
				break
			}
			continue
		}

		entries = append(
			entries,
			loki.NewEntryWithCreated(
				cfg.FixedLabels,
				created,
				push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			),
		)
		if errors.Is(err, io.EOF) {
			break
		}
	}

	return entries, http.StatusNoContent, nil
}

var _ source.HandlerRoute = (*readyRoute)(nil)

func newReadyHandler() *readyRoute {
	return &readyRoute{}
}

type readyRoute struct{}

func (r *readyRoute) Path() string {
	return pathReady
}

func (r *readyRoute) Method() string {
	return http.MethodGet
}

func (r *readyRoute) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ready"))
}

func getTenantID(r *http.Request) string {
	_, ctx, _ := user.ExtractOrgIDFromHTTPRequest(r)
	tenantID, _ := tenant.TenantID(ctx)
	return tenantID
}
