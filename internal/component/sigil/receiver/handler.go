package receiver

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type handler struct {
	logger  log.Logger
	metrics *metrics

	mu          sync.RWMutex
	forwardTo   []sigil.GenerationsReceiver
	maxBodySize int64
}

func newHandler(logger log.Logger, m *metrics, forwardTo []sigil.GenerationsReceiver, maxBodySize int64) *handler {
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
	start := time.Now()

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
			h.metrics.requests.WithLabelValues("413").Inc()
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		level.Warn(h.logger).Log("msg", "failed to read request body", "err", err)
		h.metrics.requests.WithLabelValues("500").Inc()
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	req := &sigil.GenerationsRequest{
		Body:        body,
		ContentType: r.Header.Get("Content-Type"),
		OrgID:       r.Header.Get("X-Scope-OrgID"),
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs error
		resp *sigil.GenerationsResponse
	)

	for _, receiver := range forwardTo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := receiver.ExportGenerations(r.Context(), req)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = errors.Join(errs, err)
			} else if resp == nil {
				resp = r
			}
		}()
	}
	wg.Wait()

	h.metrics.latency.WithLabelValues().Observe(time.Since(start).Seconds())

	if errs != nil {
		level.Warn(h.logger).Log("msg", "failed to forward generations", "err", errs)
	}

	// If all receivers failed, return 502.
	if resp == nil && errs != nil {
		statusCode := http.StatusBadGateway
		h.metrics.requests.WithLabelValues(strconv.Itoa(statusCode)).Inc()
		h.metrics.requestBytes.WithLabelValues(strconv.Itoa(statusCode)).Add(float64(len(body)))
		http.Error(w, "failed to forward", statusCode)
		return
	}

	statusCode := http.StatusAccepted
	if resp != nil {
		statusCode = resp.StatusCode
	}
	h.metrics.requests.WithLabelValues(strconv.Itoa(statusCode)).Inc()
	h.metrics.requestBytes.WithLabelValues(strconv.Itoa(statusCode)).Add(float64(len(body)))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if resp != nil && resp.Body != nil {
		_, _ = w.Write(resp.Body)
	}
}
