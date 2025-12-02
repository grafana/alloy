package save

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
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
	logsBatch         []loki.Entry
	batchMut          sync.Mutex
	flushTimer        *time.Timer
	ctx               context.Context
	cancel            context.CancelFunc
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

	promMetricsFolder := filepath.Join(dir, "prometheus")
	if err := os.MkdirAll(promMetricsFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create prometheus metrics directory: %w", err)
	}
	c.promMetricsFolder = promMetricsFolder

	lokiLogsFolder := filepath.Join(dir, "loki")
	if err := os.MkdirAll(lokiLogsFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create loki logs directory: %w", err)
	}
	c.lokiLogsFolder = lokiLogsFolder

	// Create logs receiver
	c.logsReceiver = loki.NewLogsReceiver(loki.WithComponentID("telemetry.save"))
	
	// Initialize batching state
	c.logsBatch = make([]loki.Entry, 0, 100) // Pre-allocate for 100 entries
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.flushTimer = time.NewTimer(5 * time.Second) // Flush every 5 seconds
	c.flushTimer.Stop() // Don't start the timer yet

	// Start the log entry handler goroutine
	go c.handleLogEntries()

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
	
	// Clean shutdown: flush any pending logs and stop timer
	c.cancel()
	c.flushLogBatch()
	if c.flushTimer != nil {
		c.flushTimer.Stop()
	}

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

// Appender returns an Appender for writing metrics.
func (c *Component) Appender(ctx context.Context) storage.Appender {
	return &appender{
		component: c,
		ctx:       ctx,
	}
}

// Define the RecordType enum
type RecordType string

const (
	RecordTypeSample    RecordType = "sample"
	RecordTypeExemplar  RecordType = "exemplar"
	RecordTypeHistogram RecordType = "histogram"
)

// Sample is the base type for telemetry samples.
type Sample struct {
	RecordType RecordType        `json:"record_type"`
	Labels     map[string]string `json:"labels"`
	Timestamp  int64             `json:"timestamp"`
}

// Update ValueSample
type ValueSample struct {
	Sample
	Value float64 `json:"value,omitempty"`
}

// Update ExemplarSample
type ExemplarSample struct {
	Sample
	Exemplar *exemplar.Exemplar `json:"exemplar,omitempty"`
}

// Update HistogramSample
type HistogramSample struct {
	Sample
	Histogram      *histogram.Histogram      `json:"histogram,omitempty"`
	FloatHistogram *histogram.FloatHistogram `json:"float_histogram,omitempty"`
}

// appender implements storage.Appender for writing metrics to file.
type appender struct {
	component *Component
	ctx       context.Context
	samples   []any // Use a generic type to accommodate multiple concrete types
}

// Append adds a sample to be written in JSON format.
func (a *appender) Append(_ storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	sample := ValueSample{
		Sample: Sample{
			RecordType: RecordTypeSample,
			Labels:     l.Map(),
			Timestamp:  t,
		},
		Value: v,
	}
	a.samples = append(a.samples, sample)
	return 0, nil
}

// AppendExemplar adds an exemplar for a series in JSON format.
func (a *appender) AppendExemplar(_ storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	sample := ExemplarSample{
		Sample: Sample{
			RecordType: RecordTypeExemplar,
			Labels:     l.Map(),
		},
		Exemplar: &e,
	}
	a.samples = append(a.samples, sample)
	return 0, nil
}

// AppendHistogram adds a histogram sample in JSON format.
func (a *appender) AppendHistogram(_ storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	sample := HistogramSample{
		Sample: Sample{
			RecordType: RecordTypeHistogram,
			Labels:     l.Map(),
			Timestamp:  t,
		},
		Histogram:      h,
		FloatHistogram: fh,
	}
	a.samples = append(a.samples, sample)
	return 0, nil
}

// AppendCTZeroSample adds a CT zero sample (no-op for file appender).
// Mark unused parameters to suppress warnings
func (a *appender) AppendCTZeroSample(_ storage.SeriesRef, _ labels.Labels, _ int64, _ int64) (storage.SeriesRef, error) {
	return 0, nil
}

// AppendHistogramCTZeroSample adds a histogram CT zero sample (no-op for file appender).
// Mark unused parameters to suppress warnings
func (a *appender) AppendHistogramCTZeroSample(_ storage.SeriesRef, _ labels.Labels, _ int64, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return 0, nil
}

// Commit writes all accumulated samples to the output file.
func (a *appender) Commit() error {
	a.component.mut.RLock()
	defer a.component.mut.RUnlock()

	if len(a.samples) == 0 {
		return nil
	}

	filePath := filepath.Join(a.component.promMetricsFolder, "metrics.json")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open metrics file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			_ = level.Error(a.component.logger).Log("msg", "failed to close file", "err", closeErr)
		}
	}()

	for _, sample := range a.samples {
		jsonData, err := json.Marshal(sample)
		if err != nil {
			return fmt.Errorf("failed to marshal sample to JSON: %w", err)
		}
		if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
			return fmt.Errorf("failed to write sample to file: %w", err)
		}
	}

	a.samples = nil // Clear the buffer
	return nil
}

// Rollback discards all accumulated samples.
func (a *appender) Rollback() error {
	a.samples = a.samples[:0]
	return nil
}

// SetOptions sets the options for the appender.
func (a *appender) SetOptions(_ *storage.AppendOptions) {
	// Not implemented for this component
}

// UpdateMetadata updates the metadata for a series.
func (a *appender) UpdateMetadata(_ storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	// Not implemented for this component
	return 0, nil
}

// LogEntry represents a log entry with its metadata for JSON serialization.
type LogEntry struct {
	Timestamp          time.Time         `json:"timestamp"`
	Line               string            `json:"line"`
	Labels             map[string]string `json:"labels"`
	StructuredMetadata []LabelPair       `json:"structured_metadata,omitempty"`
	Parsed             []LabelPair       `json:"parsed,omitempty"`
}

// LabelPair represents a key-value pair for labels.
type LabelPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// handleLogEntries processes incoming log entries and batches them for efficient writing.
func (c *Component) handleLogEntries() {
	const maxBatchSize = 50 // Max entries per batch
	const flushInterval = 5 * time.Second
	
	for {
		select {
		case entry := <-c.logsReceiver.Chan():
			c.addLogToBatch(entry)
			
			c.batchMut.Lock()
			batchSize := len(c.logsBatch)
			c.batchMut.Unlock()
			
			// Flush if batch is full
			if batchSize >= maxBatchSize {
				c.flushLogBatch()
			}
			
		case <-c.flushTimer.C:
			// Periodic flush
			c.flushLogBatch()
			
		case <-c.ctx.Done():
			// Component is shutting down
			c.flushLogBatch()
			return
		}
	}
}

// addLogToBatch adds a log entry to the current batch.
func (c *Component) addLogToBatch(entry loki.Entry) {
	c.batchMut.Lock()
	defer c.batchMut.Unlock()
	
	c.logsBatch = append(c.logsBatch, entry)
	
	// Start flush timer if this is the first entry in the batch
	if len(c.logsBatch) == 1 {
		c.flushTimer.Reset(5 * time.Second)
	}
}

// flushLogBatch writes all batched log entries to disk and clears the batch.
func (c *Component) flushLogBatch() {
	c.batchMut.Lock()
	defer c.batchMut.Unlock()
	
	if len(c.logsBatch) == 0 {
		return
	}
	
	c.mut.RLock()
	lokiLogsFolder := c.lokiLogsFolder
	c.mut.RUnlock()
	
	// Write all entries in the batch
	filePath := filepath.Join(lokiLogsFolder, "logs.json")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		_ = level.Error(c.logger).Log("msg", "failed to open logs file", "err", err)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			_ = level.Error(c.logger).Log("msg", "failed to close logs file", "err", closeErr)
		}
	}()
	
	for _, entry := range c.logsBatch {
		// Convert loki.Entry to our JSON-serializable format
		logEntry := LogEntry{
			Timestamp: entry.Timestamp,
			Line:      entry.Line,
			Labels:    make(map[string]string),
		}

		// Convert model.LabelSet to map[string]string
		for k, v := range entry.Labels {
			logEntry.Labels[string(k)] = string(v)
		}

		// Convert structured metadata
		for _, label := range entry.StructuredMetadata {
			logEntry.StructuredMetadata = append(logEntry.StructuredMetadata, LabelPair{
				Name:  label.Name,
				Value: label.Value,
			})
		}

		// Convert parsed labels
		for _, label := range entry.Parsed {
			logEntry.Parsed = append(logEntry.Parsed, LabelPair{
				Name:  label.Name,
				Value: label.Value,
			})
		}

		jsonData, err := json.Marshal(logEntry)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "failed to marshal log entry to JSON", "err", err)
			continue
		}

		if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
			_ = level.Error(c.logger).Log("msg", "failed to write log entry to file", "err", err)
			break
		}
	}
	
	// Clear the batch
	c.logsBatch = c.logsBatch[:0]
	
	// Stop the flush timer
	c.flushTimer.Stop()
}
