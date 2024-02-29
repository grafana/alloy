package app_agent_receiver

import (
	"context"
	"sync"

	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/cors"
	"golang.org/x/time/rate"
)

const apiKeyHeader = "x-api-key"

type AppAgentReceiverExporter interface {
	Name() string
	Export(ctx context.Context, payload Payload) error
}

// AppAgentReceiverHandler struct controls the data ingestion http handler of the receiver
type AppAgentReceiverHandler struct {
	exporters               []AppAgentReceiverExporter
	config                  *Config
	rateLimiter             *rate.Limiter
	exporterErrorsCollector *prometheus.CounterVec
}

// NewAppAgentReceiverHandler creates a new AppReceiver instance based on the given configuration
func NewAppAgentReceiverHandler(conf *Config, exporters []AppAgentReceiverExporter, reg prometheus.Registerer) AppAgentReceiverHandler {
	var rateLimiter *rate.Limiter
	if conf.Server.RateLimiting.Enabled {
		var rps float64
		if conf.Server.RateLimiting.RPS > 0 {
			rps = conf.Server.RateLimiting.RPS
		}

		var b int
		if conf.Server.RateLimiting.Burstiness > 0 {
			b = conf.Server.RateLimiting.Burstiness
		}
		rateLimiter = rate.NewLimiter(rate.Limit(rps), b)
	}

	exporterErrorsCollector := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_agent_receiver_exporter_errors_total",
		Help: "Total number of errors produced by a receiver exporter",
	}, []string{"exporter"})

	reg.MustRegister(exporterErrorsCollector)

	return AppAgentReceiverHandler{
		exporters:               exporters,
		config:                  conf,
		rateLimiter:             rateLimiter,
		exporterErrorsCollector: exporterErrorsCollector,
	}
}

// HTTPHandler is the http.Handler for the receiver. It will do the following
// 0. Enable CORS for the configured hosts
// 1. Check if the request should be rate limited
// 2. Verify that the payload size is within limits
// 3. Start two go routines for exporters processing and exporting data respectively
// 4. Respond with 202 once all the work is done
func (ar *AppAgentReceiverHandler) HTTPHandler(logger log.Logger) http.Handler {
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check rate limiting state
		if ar.config.Server.RateLimiting.Enabled {
			if ok := ar.rateLimiter.Allow(); !ok {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
		}

		// check API key if one is provided
		if len(ar.config.Server.APIKey) > 0 && subtle.ConstantTimeCompare([]byte(r.Header.Get(apiKeyHeader)), []byte(ar.config.Server.APIKey)) == 0 {
			http.Error(w, "api key not provided or incorrect", http.StatusUnauthorized)
			return
		}

		// Verify content length. We trust net/http to give us the correct number
		if ar.config.Server.MaxAllowedPayloadSize > 0 && r.ContentLength > ar.config.Server.MaxAllowedPayloadSize {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}

		var p Payload
		err := json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var wg sync.WaitGroup

		for _, exporter := range ar.exporters {
			wg.Add(1)
			go func(exp AppAgentReceiverExporter) {
				defer wg.Done()
				if err := exp.Export(r.Context(), p); err != nil {
					level.Error(logger).Log("msg", "exporter error", "exporter", exp.Name(), "error", err)
					ar.exporterErrorsCollector.WithLabelValues(exp.Name()).Inc()
				}
			}(exporter)
		}

		wg.Wait()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})

	if len(ar.config.Server.CORSAllowedOrigins) > 0 {
		c := cors.New(cors.Options{
			AllowedOrigins: ar.config.Server.CORSAllowedOrigins,
			AllowedHeaders: []string{apiKeyHeader, "content-type", "x-faro-session-id"},
		})
		handler = c.Handler(handler)
	}

	return handler
}
