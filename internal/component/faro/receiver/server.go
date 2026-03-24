package receiver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/dskit/instrument"
	"github.com/grafana/dskit/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

type serverMetrics struct {
	requestDuration  *prometheus.HistogramVec
	rxMessageSize    *prometheus.HistogramVec
	txMessageSize    *prometheus.HistogramVec
	inflightRequests *prometheus.GaugeVec
}

func newServerMetrics(reg prometheus.Registerer) *serverMetrics {
	m := &serverMetrics{
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "faro_receiver_request_duration_seconds",
			Help:    "Time (in seconds) spent serving HTTP requests.",
			Buckets: instrument.DefBuckets,
		}, []string{"method", "route", "status_code", "ws"}),

		rxMessageSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "faro_receiver_request_message_bytes",
			Help:    "Size (in bytes) of messages received in the request.",
			Buckets: middleware.BodySizeBuckets,
		}, []string{"method", "route"}),

		txMessageSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "faro_receiver_response_message_bytes",
			Help:    "Size (in bytes) of messages sent in response.",
			Buckets: middleware.BodySizeBuckets,
		}, []string{"method", "route"}),

		inflightRequests: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "faro_receiver_inflight_requests",
			Help: "Current number of inflight requests.",
		}, []string{"method", "route"}),
	}

	m.requestDuration = util.MustRegisterOrGet(reg, m.requestDuration).(*prometheus.HistogramVec)
	m.rxMessageSize = util.MustRegisterOrGet(reg, m.rxMessageSize).(*prometheus.HistogramVec)
	m.txMessageSize = util.MustRegisterOrGet(reg, m.txMessageSize).(*prometheus.HistogramVec)
	m.inflightRequests = util.MustRegisterOrGet(reg, m.inflightRequests).(*prometheus.GaugeVec)

	return m
}

// server represents the HTTP server for which the receiver receives metrics.
// server is not dynamically updatable. To update server, shut down the old
// server and start a new one.
type server struct {
	log     log.Logger
	args    ServerArguments
	handler http.Handler
	metrics *serverMetrics
}

func newServer(l log.Logger, args ServerArguments, metrics *serverMetrics, h http.Handler) *server {
	return &server{
		log:     l,
		args:    args,
		handler: h,
		metrics: metrics,
	}
}

func (s *server) Run(ctx context.Context) error {
	r := mux.NewRouter()
	r.Handle("/collect", s.handler).Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	mw := middleware.Instrument{
		Duration:         s.metrics.requestDuration,
		RequestBodySize:  s.metrics.rxMessageSize,
		ResponseBodySize: s.metrics.txMessageSize,
		InflightRequests: s.metrics.inflightRequests,
	}

	ri := middleware.RouteInjector{
		RouteMatcher: r,
	}
	riHandler := ri.Wrap(r)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.args.Host, s.args.Port),
		Handler: mw.Wrap(riHandler),
	}

	errCh := make(chan error, 1)
	go func() {
		level.Info(s.log).Log("msg", "starting server", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		level.Info(s.log).Log("msg", "terminating server")

		if err := srv.Shutdown(ctx); err != nil {
			level.Error(s.log).Log("msg", "failed to gracefully terminate server", "err", err)
		}

	case err := <-errCh:
		return err
	}

	return nil
}
