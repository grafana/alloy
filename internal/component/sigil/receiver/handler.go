package receiver

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/grafana/alloy/internal/component/sigil"
	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"github.com/grafana/sigil-sdk/go/proto/sigil/wire"
)

// fanOutMetrics adapts the package metrics to sigil.FanOutMetrics.
func (h *handler) fanOutMetrics() sigil.FanOutMetrics {
	return sigil.FanOutMetrics{PartialFailures: h.metrics.partialFailures}
}

type handler struct {
	logger  *slog.Logger
	metrics *metrics

	mu          sync.RWMutex
	forwardTo   []sigil.GenerationsReceiver
	maxBodySize int64
}

func newHandler(logger *slog.Logger, m *metrics, forwardTo []sigil.GenerationsReceiver, maxBodySize int64) *handler {
	return &handler{
		logger:      logger,
		metrics:     m,
		forwardTo:   forwardTo,
		maxBodySize: maxBodySize,
	}
}

func (h *handler) update(forwardTo []sigil.GenerationsReceiver, maxBodySize int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.forwardTo = forwardTo
	h.maxBodySize = maxBodySize
}

func (h *handler) getConfig() ([]sigil.GenerationsReceiver, int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.forwardTo, h.maxBodySize
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	forwardTo, maxBodySize := h.getConfig()
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		h.logger.Warn("failed to read request body", "err", err)
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	contentType := r.Header.Get("Content-Type")
	orgID := r.Header.Get(wire.TenantHeaderName)

	req, err := sigil.ParseGenerationsRequest(body, contentType, orgID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sigil.ErrUnsupportedContentType) {
			status = http.StatusUnsupportedMediaType
		}
		h.logger.Debug("failed to parse generation export request", "err", err)
		http.Error(w, err.Error(), status)
		return
	}

	resp, fanErr := sigil.FanOut(r.Context(), req, forwardTo, h.logger, h.fanOutMetrics())

	// If every branch failed, return 502.
	if resp == nil && fanErr != nil {
		h.logger.Warn("failed to forward generations", "err", fanErr)
		http.Error(w, "failed to forward", http.StatusBadGateway)
		return
	}

	statusCode := http.StatusAccepted
	if resp != nil && resp.StatusCode != 0 {
		if resp.StatusCode < 100 || resp.StatusCode > 999 {
			h.logger.Warn("invalid downstream status code", "status_code", resp.StatusCode)
		} else {
			statusCode = resp.StatusCode
		}
	}

	var respProto *sigilv1.ExportGenerationsResponse
	if resp != nil {
		respProto = resp.Response
	}
	respBody, marshalErr := sigil.MarshalGenerationsResponse(respProto)
	if marshalErr != nil {
		h.logger.Warn("failed to marshal response", "err", marshalErr)
		http.Error(w, "failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", wire.ContentTypeJSON)
	w.WriteHeader(statusCode)
	_, _ = w.Write(respBody)
}
