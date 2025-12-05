package save

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kit/log/level"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/grafana/alloy/internal/component/otelcol"
)

// otlpConsumer implements otelcol.Consumer (all three: Logs, Metrics, Traces) for saving OTLP data to files.
type otlpConsumer struct {
	component        *Component
	logsMarshaler    *plog.JSONMarshaler
	metricsMarshaler *pmetric.JSONMarshaler
	tracesMarshaler  *ptrace.JSONMarshaler
}

// newOTLPConsumer creates a new combined OTLP consumer.
func newOTLPConsumer(component *Component) otelcol.Consumer {
	return &otlpConsumer{
		component:        component,
		logsMarshaler:    &plog.JSONMarshaler{},
		metricsMarshaler: &pmetric.JSONMarshaler{},
		tracesMarshaler:  &ptrace.JSONMarshaler{},
	}
}

// Capabilities returns the consumer capabilities.
func (c *otlpConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs saves the OTLP logs to a file.
func (c *otlpConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	c.component.mut.RLock()
	otlpLogsFolder := c.component.otlpLogsFolder
	c.component.mut.RUnlock()

	jsonData, err := c.logsMarshaler.MarshalLogs(logs)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP logs to JSON: %w", err)
	}

	filePath := filepath.Join(otlpLogsFolder, "logs.json")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open OTLP logs file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			_ = level.Error(c.component.logger).Log("msg", "failed to close OTLP logs file", "err", closeErr)
		}
	}()

	if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write OTLP logs to file: %w", err)
	}

	return nil
}

// ConsumeMetrics saves the OTLP metrics to a file.
func (c *otlpConsumer) ConsumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	c.component.mut.RLock()
	otlpMetricsFolder := c.component.otlpMetricsFolder
	c.component.mut.RUnlock()

	jsonData, err := c.metricsMarshaler.MarshalMetrics(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP metrics to JSON: %w", err)
	}

	filePath := filepath.Join(otlpMetricsFolder, "metrics.json")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open OTLP metrics file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			_ = level.Error(c.component.logger).Log("msg", "failed to close OTLP metrics file", "err", closeErr)
		}
	}()

	if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write OTLP metrics to file: %w", err)
	}

	return nil
}

// ConsumeTraces saves the OTLP traces to a file.
func (c *otlpConsumer) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	c.component.mut.RLock()
	otlpTracesFolder := c.component.otlpTracesFolder
	c.component.mut.RUnlock()

	jsonData, err := c.tracesMarshaler.MarshalTraces(traces)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP traces to JSON: %w", err)
	}

	filePath := filepath.Join(otlpTracesFolder, "traces.json")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open OTLP traces file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			_ = level.Error(c.component.logger).Log("msg", "failed to close OTLP traces file", "err", closeErr)
		}
	}()

	if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write OTLP traces to file: %w", err)
	}

	return nil
}
