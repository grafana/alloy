package save

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "telemetry.save",
		Args:      Arguments{},
		Exports:   Exports{},
		Stability: featuregate.StabilityExperimental,
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewComponent(opts, args.(Arguments))
		},
	})
}

// Arguments configures the telemetry.save component.
type Arguments struct {
	OutputLocation string `alloy:"output_location,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{OutputLocation: "telemetry/save/"}
}

// Exports are the set of fields exposed by the telemetry.save component.
type Exports struct {
	MetricsReceiver storage.Appendable      `alloy:"metrics_receiver,attr"`
	LogsReceiver    loki.LogsReceiver       `alloy:"logs_receiver,attr"`
	OTLP            otelcol.ConsumerExports `alloy:"otlp,attr"`
}

// Component is the telemetry.save component.
type Component struct {
	mut    sync.RWMutex
	args   Arguments
	logger log.Logger

	promMetricsFolder string

	lokiLogsFolder string
	logsReceiver   loki.LogsReceiver
	logsHandler    *LogsHandler

	otlpLogsFolder    string
	otlpMetricsFolder string
	otlpTracesFolder  string
	otlpConsumer      otelcol.Consumer
}

var _ component.Component = (*Component)(nil)

// NewComponent creates a new telemetry.save component.
func NewComponent(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		args:   args,
		logger: opts.Logger,
	}

	level.Info(c.logger).Log("msg", "initializing telemetry.save component", "output_location", args.OutputLocation)

	// Ensure the output directory exists and is clean
	dir := filepath.Dir(args.OutputLocation)
	if _, err := os.Stat(dir); err == nil {
		// Directory exists, clear it
		if err := os.RemoveAll(dir); err != nil {
			return nil, fmt.Errorf("failed to clear existing output directory: %w", err)
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create prometheus metrics folder
	promMetricsFolder := filepath.Join(dir, "prometheus")
	if err := os.MkdirAll(promMetricsFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create prometheus metrics directory: %w", err)
	}
	c.promMetricsFolder = promMetricsFolder

	// Create loki logs folder
	lokiLogsFolder := filepath.Join(dir, "loki")
	if err := os.MkdirAll(lokiLogsFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create loki logs directory: %w", err)
	}
	c.lokiLogsFolder = lokiLogsFolder

	// Create OTLP folder
	otlpFolder := filepath.Join(dir, "otlp")
	if err := os.MkdirAll(otlpFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create otlp directory: %w", err)
	}

	// Create OTLP logs folder
	otlpLogsFolder := filepath.Join(otlpFolder, "logs")
	if err := os.MkdirAll(otlpLogsFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create otlp logs directory: %w", err)
	}
	c.otlpLogsFolder = otlpLogsFolder

	// Create OTLP metrics folder
	otlpMetricsFolder := filepath.Join(otlpFolder, "metrics")
	if err := os.MkdirAll(otlpMetricsFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create otlp metrics directory: %w", err)
	}
	c.otlpMetricsFolder = otlpMetricsFolder

	// Create OTLP traces folder
	otlpTracesFolder := filepath.Join(otlpFolder, "traces")
	if err := os.MkdirAll(otlpTracesFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create otlp traces directory: %w", err)
	}
	c.otlpTracesFolder = otlpTracesFolder

	// Create logs receiver
	c.logsReceiver = loki.NewLogsReceiver(loki.WithComponentID("telemetry.save"))

	// Initialize logs handler
	c.logsHandler = NewLogsHandler(c)

	// Start the log entry handler goroutine
	c.logsHandler.Start(c.logsReceiver)

	// Initialize OTLP consumer
	c.otlpConsumer = newOTLPConsumer(c)

	// Export the receiver interfaces
	opts.OnStateChange(Exports{
		MetricsReceiver: c,
		LogsReceiver:    c.logsReceiver,
		OTLP: otelcol.ConsumerExports{
			Input: c.otlpConsumer,
		},
	})

	return c, nil
}

// Run starts the component, blocking until ctx is canceled.
func (c *Component) Run(ctx context.Context) error {
	_ = level.Info(c.logger).Log("msg", "telemetry.save component started", "output_location", c.args.OutputLocation)

	<-ctx.Done()

	// Clean shutdown: stop logs handler
	c.logsHandler.Stop()

	_ = level.Info(c.logger).Log("msg", "telemetry.save component stopped")
	return nil
}

// Update provides a new config to the component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	// Check if output location changed
	if newArgs.OutputLocation == c.args.OutputLocation {
		return nil
	}

	// Ensure the new output directory exists and is clean
	dir := filepath.Dir(newArgs.OutputLocation)
	if _, err := os.Stat(dir); err == nil {
		// Directory exists, clear it
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("failed to clear existing output directory: %w", err)
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Update prometheus and loki folders
	promMetricsFolder := filepath.Join(dir, "prometheus")
	if err := os.MkdirAll(promMetricsFolder, 0755); err != nil {
		return fmt.Errorf("failed to create prometheus metrics directory: %w", err)
	}
	c.promMetricsFolder = promMetricsFolder

	lokiLogsFolder := filepath.Join(dir, "loki")
	if err := os.MkdirAll(lokiLogsFolder, 0755); err != nil {
		return fmt.Errorf("failed to create loki logs directory: %w", err)
	}
	c.lokiLogsFolder = lokiLogsFolder

	// Update OTLP folders
	otlpFolder := filepath.Join(dir, "otlp")
	if err := os.MkdirAll(otlpFolder, 0755); err != nil {
		return fmt.Errorf("failed to create otlp directory: %w", err)
	}

	otlpLogsFolder := filepath.Join(otlpFolder, "logs")
	if err := os.MkdirAll(otlpLogsFolder, 0755); err != nil {
		return fmt.Errorf("failed to create otlp logs directory: %w", err)
	}
	c.otlpLogsFolder = otlpLogsFolder

	otlpMetricsFolder := filepath.Join(otlpFolder, "metrics")
	if err := os.MkdirAll(otlpMetricsFolder, 0755); err != nil {
		return fmt.Errorf("failed to create otlp metrics directory: %w", err)
	}
	c.otlpMetricsFolder = otlpMetricsFolder

	otlpTracesFolder := filepath.Join(otlpFolder, "traces")
	if err := os.MkdirAll(otlpTracesFolder, 0755); err != nil {
		return fmt.Errorf("failed to create otlp traces directory: %w", err)
	}
	c.otlpTracesFolder = otlpTracesFolder

	// Cleanup the old directory
	oldDir := filepath.Dir(c.args.OutputLocation)
	if err := os.RemoveAll(oldDir); err != nil {
		level.Warn(c.logger).Log("msg", "failed to remove old output directory", "dir", oldDir, "err", err)
	}

	c.args = newArgs
	level.Info(c.logger).Log("msg", "telemetry.save component updated", "output_location", c.args.OutputLocation)
	return nil
}
