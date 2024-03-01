package remotewriteexporter

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
)

const (
	// TypeStr is the unique identifier for the Prometheus remote write exporter.
	// TODO: Rename to walexporter (?). Remote write makes no sense, it appends to a WAL.
	TypeStr = "remote_write"
)

// Config holds the configuration for the Prometheus remote write processor.
type Config struct {
	ConstLabels  prometheus.Labels `mapstructure:"const_labels"`
	Namespace    string            `mapstructure:"namespace"`
	PromInstance string            `mapstructure:"metrics_instance"`
	// StaleTime is the duration after which a series is considered stale and will be removed.
	StaleTime time.Duration `mapstructure:"stale_time"`
	// LoopInterval is the duration after which the exporter will be checked for new data.
	// New data is flushed to a WAL.
	LoopInterval time.Duration `mapstructure:"loop_interval"`
}

// NewFactory returns a new factory for the Prometheus remote write processor.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		TypeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelUndefined),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsExporter(
	_ context.Context,
	_ exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {

	eCfg := cfg.(*Config)
	return newRemoteWriteExporter(eCfg)
}
