package receiver

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/cors"
	"go.opentelemetry.io/collector/client"
	"golang.org/x/time/rate"
)

const apiKeyHeader = "x-api-key"

var defaultAllowedHeaders = []string{"content-type", "traceparent", apiKeyHeader, "x-faro-session-id", "x-scope-orgid"}

type handler struct {
	log         log.Logger
	rateLimiter *rate.Limiter
	exporters   []exporter
	errorsTotal *prometheus.CounterVec

	argsMut sync.RWMutex
	args    ServerArguments
	cors    *cors.Cors
}

var _ http.Handler = (*handler)(nil)

func newHandler(l log.Logger, reg prometheus.Registerer, exporters []exporter) *handler {
	errorsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faro_receiver_exporter_errors_total",
		Help: "Total number of errors produced by a receiver exporter",
	}, []string{"exporter"})
	errorsTotal = util.MustRegisterOrGet(reg, errorsTotal).(*prometheus.CounterVec)

	return &handler{
		log:         l,
		rateLimiter: rate.NewLimiter(rate.Inf, 0),
		exporters:   exporters,
		errorsTotal: errorsTotal,
	}
}

func (h *handler) Update(args ServerArguments) {
	h.argsMut.Lock()
	defer h.argsMut.Unlock()

	h.args = args

	if args.RateLimiting.Enabled {
		// Updating the rate limit to time.Now() would immediately fill the
		// buckets. To allow requsts to immediately pass through, we adjust the
		// time to set the limit/burst to to allow for both the normal rate and
		// burst to be filled.
		t := time.Now().Add(-time.Duration(float64(time.Second) * args.RateLimiting.Rate * args.RateLimiting.BurstSize))

		h.rateLimiter.SetLimitAt(t, rate.Limit(args.RateLimiting.Rate))
		h.rateLimiter.SetBurstAt(t, int(args.RateLimiting.BurstSize))
	} else {
		// Set to infinite rate limit.
		h.rateLimiter.SetLimit(rate.Inf)
		h.rateLimiter.SetBurst(0) // 0 burst is ignored when using rate.Inf.
	}

	if len(args.CORSAllowedOrigins) > 0 {
		h.cors = cors.New(cors.Options{
			AllowedOrigins: args.CORSAllowedOrigins,
			AllowedHeaders: defaultAllowedHeaders,
		})
	} else {
		h.cors = nil // Disable cors.
	}
}

func (h *handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.argsMut.RLock()
	defer h.argsMut.RUnlock()

	// Propagate request headers as metadata
	if h.args.IncludeMetadata {
		cl := client.FromContext(req.Context())
		cl.Metadata = client.NewMetadata(req.Header.Clone())
		req = req.WithContext(client.NewContext(req.Context(), cl))
	}

	if h.cors != nil {
		h.cors.ServeHTTP(rw, req, h.handleRequest)
	} else {
		h.handleRequest(rw, req)
	}
}

func (h *handler) handleRequest(rw http.ResponseWriter, req *http.Request) {
	if !h.rateLimiter.Allow() {
		http.Error(rw, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}

	// If an API key is configured, ensure the request has a matching key.
	if len(h.args.APIKey) > 0 {
		apiHeader := req.Header.Get(apiKeyHeader)

		if subtle.ConstantTimeCompare([]byte(apiHeader), []byte(h.args.APIKey)) != 1 {
			http.Error(rw, "API key not provided or incorrect", http.StatusUnauthorized)
			return
		}
	}

	// Validate content length.
	if h.args.MaxAllowedPayloadSize > 0 && req.ContentLength > int64(h.args.MaxAllowedPayloadSize) {
		http.Error(rw, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return
	}

	var p payload.Payload
	if err := json.NewDecoder(req.Body).Decode(&p); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	var wg sync.WaitGroup
	for _, exp := range h.exporters {
		wg.Add(1)
		go func(exp exporter) {
			defer wg.Done()

			if err := exp.Export(req.Context(), p); err != nil {
				level.Error(h.log).Log("msg", "exporter failed with error", "exporter", exp.Name(), "err", err)
				h.errorsTotal.WithLabelValues(exp.Name()).Inc()
			}
		}(exp)
	}
	wg.Wait()

	rw.WriteHeader(http.StatusAccepted)
	_, _ = rw.Write([]byte("ok"))
}
