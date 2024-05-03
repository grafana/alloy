package automaticloggingprocessor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	util_log "github.com/grafana/alloy/internal/util/log"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/atomic"
)

const (
	defaultLogsTag     = "traces"
	defaultServiceKey  = "svc"
	defaultSpanNameKey = "span"
	defaultStatusKey   = "status"
	defaultDurationKey = "dur"
	defaultTraceIDKey  = "tid"

	defaultTimeout = time.Millisecond
)

type automaticLoggingProcessor struct {
	nextConsumer consumer.Traces

	cfg         *AutomaticLoggingConfig
	logToStdout bool
	done        atomic.Bool

	labels map[string]struct{}

	logger log.Logger
}

func newTraceProcessor(nextConsumer consumer.Traces, cfg *AutomaticLoggingConfig) (processor.Traces, error) {
	logger := log.With(util_log.Logger, "component", "traces automatic logging")

	if nextConsumer == nil {
		return nil, errors.New("nil next Consumer")
	}

	if !cfg.Roots && !cfg.Processes && !cfg.Spans {
		return nil, errors.New("automaticLoggingProcessor requires one of roots, processes, or spans to be enabled")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}

	if cfg.Backend == "" {
		cfg.Backend = BackendStdout
	}

	if cfg.Backend != BackendLogs && cfg.Backend != BackendStdout {
		return nil, fmt.Errorf("automaticLoggingProcessor requires a backend of type '%s' or '%s'", BackendLogs, BackendStdout)
	}

	logToStdout := false
	if cfg.Backend == BackendStdout {
		logToStdout = true
	}

	cfg.Overrides.LogsTag = override(cfg.Overrides.LogsTag, defaultLogsTag)
	cfg.Overrides.ServiceKey = override(cfg.Overrides.ServiceKey, defaultServiceKey)
	cfg.Overrides.SpanNameKey = override(cfg.Overrides.SpanNameKey, defaultSpanNameKey)
	cfg.Overrides.StatusKey = override(cfg.Overrides.StatusKey, defaultStatusKey)
	cfg.Overrides.DurationKey = override(cfg.Overrides.DurationKey, defaultDurationKey)
	cfg.Overrides.TraceIDKey = override(cfg.Overrides.TraceIDKey, defaultTraceIDKey)

	labels := make(map[string]struct{}, len(cfg.Labels))
	for _, l := range cfg.Labels {
		labels[l] = struct{}{}
	}

	return &automaticLoggingProcessor{
		nextConsumer: nextConsumer,
		cfg:          cfg,
		logToStdout:  logToStdout,
		logger:       logger,
		done:         atomic.Bool{},
		labels:       labels,
	}, nil
}

func (p *automaticLoggingProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}

func (p *automaticLoggingProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

// Start is invoked during service startup.
func (p *automaticLoggingProcessor) Start(ctx context.Context, _ component.Host) error {
	// NOTE(rfratto): automaticloggingprocesor only exists for config conversions
	// so we don't need any logic here.
	return nil
}

// Shutdown is invoked during service shutdown.
func (p *automaticLoggingProcessor) Shutdown(context.Context) error {
	p.done.Store(true)

	return nil
}

func override(cfgValue string, defaultValue string) string {
	if cfgValue == "" {
		return defaultValue
	}
	return cfgValue
}
