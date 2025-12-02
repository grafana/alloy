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
	Receiver     storage.Appendable `alloy:"receiver,attr"`
	LogsReceiver loki.LogsReceiver  `alloy:"logs_receiver,attr"`
}

// Component is the telemetry.save component.
type Component struct {
	mut               sync.RWMutex
	args              Arguments
	logger            log.Logger
	promMetricsFolder string
	lokiLogsFolder    string
	logsReceiver      loki.LogsReceiver
	logsHandler       *LogsHandler
}

var _ component.Component = (*Component)(nil)

// NewComponent creates a new telemetry.save component.
func NewComponent(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		args:   args,
		logger: opts.Logger,
	}

	level.Info(c.logger).Log("msg", "initializing telemetry.save component", "output_location", args.OutputLocation)

	// Ensure the output directory exists
	dir := filepath.Dir(args.OutputLocation)
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

	// Create logs receiver
	c.logsReceiver = loki.NewLogsReceiver(loki.WithComponentID("telemetry.save"))
	
	// Initialize logs handler
	c.logsHandler = NewLogsHandler(c)
	
	// Start the log entry handler goroutine
	c.logsHandler.Start(c.logsReceiver)

	// Export the receiver interfaces
	opts.OnStateChange(Exports{
		Receiver:     c,
		LogsReceiver: c.logsReceiver,
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

	// Ensure the new output directory exists
	dir := filepath.Dir(newArgs.OutputLocation)
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

	// Cleanup the old directory
	oldDir := filepath.Dir(c.args.OutputLocation)
	if err := os.RemoveAll(oldDir); err != nil {
		level.Warn(c.logger).Log("msg", "failed to remove old output directory", "dir", oldDir, "err", err)
	}

	c.args = newArgs
	level.Info(c.logger).Log("msg", "telemetry.save component updated", "output_location", c.args.OutputLocation)
	return nil
}