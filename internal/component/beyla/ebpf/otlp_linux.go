//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// startOTLPReceiver starts an HTTP server to receive OTLP traces and/or metrics from Beyla
// and forwards them to the configured Output consumers.
func (c *Component) startOTLPReceiver() error {
	if c.args.Output == nil || (len(c.args.Output.Traces) == 0 && len(c.args.Output.Metrics) == 0) {
		return nil
	}

	port, err := findFreePort()
	if err != nil {
		return fmt.Errorf("failed to allocate OTLP receiver port: %w", err)
	}

	c.mut.Lock()
	c.otlpReceiverPort = port
	c.mut.Unlock()

	level.Info(c.opts.Logger).Log("msg", "starting OTLP receiver", "port", port)

	mux := http.NewServeMux()
	if len(c.args.Output.Traces) > 0 {
		mux.HandleFunc("/v1/traces", c.handleOTLPTraces)
	}
	if len(c.args.Output.Metrics) > 0 {
		mux.HandleFunc("/v1/metrics", c.handleOTLPMetrics)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	c.mut.Lock()
	c.otlpServer = server
	c.mut.Unlock()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			level.Error(c.opts.Logger).Log("msg", "OTLP receiver server error", "err", err)
		}
	}()

	return nil
}

func (c *Component) stopOTLPReceiver() {
	c.mut.Lock()
	server := c.otlpServer
	c.otlpServer = nil
	c.mut.Unlock()

	if server != nil {
		level.Debug(c.opts.Logger).Log("msg", "stopping OTLP receiver")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			level.Warn(c.opts.Logger).Log("msg", "error shutting down OTLP receiver", "err", err)
		}
	}
}

func (c *Component) handleOTLPMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to read OTLP request body", "err", err)
		http.Error(w, "failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := pmetricotlp.NewExportRequest()
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		err = req.UnmarshalJSON(body)
	} else {
		err = req.UnmarshalProto(body)
	}
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to unmarshal OTLP metrics", "err", err)
		http.Error(w, "failed to parse OTLP request", http.StatusBadRequest)
		return
	}

	metrics := req.Metrics()

	c.mut.Lock()
	consumers := c.args.Output.Metrics
	c.mut.Unlock()

	for _, consumer := range consumers {
		if err := consumer.ConsumeMetrics(r.Context(), metrics); err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to forward metrics to consumer", "err", err)
			http.Error(w, "failed to process metrics", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}"))
}

func (c *Component) handleOTLPTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to read OTLP request body", "err", err)
		http.Error(w, "failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := ptraceotlp.NewExportRequest()
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		err = req.UnmarshalJSON(body)
	} else {
		err = req.UnmarshalProto(body)
	}
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to unmarshal OTLP traces", "err", err)
		http.Error(w, "failed to parse OTLP request", http.StatusBadRequest)
		return
	}

	traces := req.Traces()

	c.mut.Lock()
	consumers := c.args.Output.Traces
	c.mut.Unlock()

	for _, consumer := range consumers {
		if err := consumer.ConsumeTraces(r.Context(), traces); err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to forward traces to consumer", "err", err)
			http.Error(w, "failed to process traces", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}"))
}
