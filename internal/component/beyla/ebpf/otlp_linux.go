//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/grafana/alloy/internal/component/otelcol"
)

const (
	otlpQueueCapacity = 256
	otlpWorkers       = 4
)

type otlpItem struct {
	isTraces    bool
	body        []byte
	contentType string
}

func (c *Component) startOTLPReceiver() error {
	output := c.args.Output

	if output == nil || (len(output.Traces) == 0 && len(output.Metrics) == 0) {
		return nil
	}

	addr := abstractSocketAddr("otlp", c.opts.ID)
	lis, err := net.Listen("unix", addr)

	if err != nil {
		return fmt.Errorf("failed to listen on OTLP receiver UDS %q: %w", addr, err)
	}

	queue := make(chan otlpItem, otlpQueueCapacity)
	workerCtx, workerCancel := context.WithCancel(context.Background())

	for i := 0; i < otlpWorkers; i++ {
		c.otlpWorkersWG.Add(1)
		go c.otlpWorker(workerCtx, queue, output)
	}

	c.opts.Logger.Info("starting OTLP receiver", "addr", addr)

	mux := http.NewServeMux()

	if len(output.Traces) > 0 {
		mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
			c.enqueueOTLP(w, r, queue, true)
		})
	}

	if len(output.Metrics) > 0 {
		mux.HandleFunc("/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
			c.enqueueOTLP(w, r, queue, false)
		})
	}

	server := &http.Server{Handler: mux}

	c.otlpReceiverAddr = addr
	c.otlpQueue = queue
	c.otlpWorkerCancel = workerCancel
	c.otlpServer = server

	go func() {
		if err := server.Serve(lis); err != nil && err != http.ErrServerClosed {
			c.opts.Logger.Error("OTLP receiver server error", "err", err)
		}
	}()

	return nil
}

func (c *Component) stopOTLPReceiver() {
	server := c.otlpServer
	queue := c.otlpQueue
	cancel := c.otlpWorkerCancel

	c.otlpServer = nil
	c.otlpQueue = nil
	c.otlpWorkerCancel = nil

	if server != nil {
		c.opts.Logger.Debug("stopping OTLP receiver")
		ctx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(ctx); err != nil {
			c.opts.Logger.Warn("error shutting down OTLP receiver", "err", err)
		}
	}

	if cancel != nil {
		cancel()
	}

	if queue != nil {
		close(queue)
	}

	c.otlpWorkersWG.Wait()
}

func (c *Component) enqueueOTLP(w http.ResponseWriter, r *http.Request, queue chan otlpItem, isTraces bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)

	if err != nil {
		c.opts.Logger.Error("failed to read OTLP request body", "err", err)
		http.Error(w, "failed to read request", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	item := otlpItem{isTraces: isTraces, body: body, contentType: r.Header.Get("Content-Type")}

	select {
	case queue <- item:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	default:
		// Sustained downstream backpressure; 503 is retryable and Beyla's OTLP exporter honours it.
		c.opts.Logger.Warn("OTLP receiver queue full, rejecting request", "is_traces", isTraces)
		http.Error(w, "receiver overloaded", http.StatusServiceUnavailable)
	}
}

func (c *Component) otlpWorker(ctx context.Context, queue <-chan otlpItem, output *otelcol.ConsumerArguments) {
	defer c.otlpWorkersWG.Done()

	for item := range queue {
		if item.isTraces {
			c.consumeTraces(ctx, item, output.Traces)
		} else {
			c.consumeMetrics(ctx, item, output.Metrics)
		}
	}
}

func (c *Component) consumeMetrics(ctx context.Context, item otlpItem, consumers []otelcol.Consumer) {
	req := pmetricotlp.NewExportRequest()
	var err error

	if strings.Contains(item.contentType, "application/json") {
		err = req.UnmarshalJSON(item.body)
	} else {
		err = req.UnmarshalProto(item.body)
	}

	if err != nil {
		c.opts.Logger.Error("failed to unmarshal OTLP metrics", "err", err)
		return
	}

	metrics := req.Metrics()

	for _, consumer := range consumers {
		if err := consumer.ConsumeMetrics(ctx, metrics); err != nil {
			c.opts.Logger.Error("failed to forward metrics to consumer", "err", err)
			return
		}
	}
}

func (c *Component) consumeTraces(ctx context.Context, item otlpItem, consumers []otelcol.Consumer) {
	req := ptraceotlp.NewExportRequest()

	var err error

	if strings.Contains(item.contentType, "application/json") {
		err = req.UnmarshalJSON(item.body)
	} else {
		err = req.UnmarshalProto(item.body)
	}

	if err != nil {
		c.opts.Logger.Error("failed to unmarshal OTLP traces", "err", err)
		return
	}

	traces := req.Traces()

	for _, consumer := range consumers {
		if err := consumer.ConsumeTraces(ctx, traces); err != nil {
			c.opts.Logger.Error("failed to forward traces to consumer", "err", err)
			return
		}
	}
}
