package save

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kit/log/level"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

// Define the RecordType enum for metrics
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

// ValueSample represents a metric sample with a value.
type ValueSample struct {
	Sample
	Value float64 `json:"value,omitempty"`
}

// ExemplarSample represents an exemplar sample.
type ExemplarSample struct {
	Sample
	Exemplar *exemplar.Exemplar `json:"exemplar,omitempty"`
}

// HistogramSample represents a histogram sample.
type HistogramSample struct {
	Sample
	Histogram      *histogram.Histogram      `json:"histogram,omitempty"`
	FloatHistogram *histogram.FloatHistogram `json:"float_histogram,omitempty"`
}

// metricsAppender implements storage.Appender for writing metrics to file.
type metricsAppender struct {
	component *Component
	ctx       context.Context
	samples   []any // Use a generic type to accommodate multiple concrete types
}

// Appender returns an Appender for writing metrics.
func (c *Component) Appender(ctx context.Context) storage.Appender {
	return &metricsAppender{
		component: c,
		ctx:       ctx,
	}
}

// Append adds a sample to be written in JSON format.
func (a *metricsAppender) Append(_ storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
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
func (a *metricsAppender) AppendExemplar(_ storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
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
func (a *metricsAppender) AppendHistogram(_ storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
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
func (a *metricsAppender) AppendCTZeroSample(_ storage.SeriesRef, _ labels.Labels, _ int64, _ int64) (storage.SeriesRef, error) {
	return 0, nil
}

// AppendHistogramCTZeroSample adds a histogram CT zero sample (no-op for file appender).
func (a *metricsAppender) AppendHistogramCTZeroSample(_ storage.SeriesRef, _ labels.Labels, _ int64, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return 0, nil
}

// Commit writes all accumulated samples to the output file.
func (a *metricsAppender) Commit() error {
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

	jsonData, err := json.Marshal(a.samples)
	if err != nil {
		return fmt.Errorf("failed to marshal sample to JSON: %w", err)
	}
	if _, err := file.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write samples to file: %w", err)
	}

	// Clear samples after successful write
	a.samples = a.samples[:0]
	return nil
}

// Rollback discards all accumulated samples.
func (a *metricsAppender) Rollback() error {
	a.samples = a.samples[:0]
	return nil
}

// SetOptions sets the options for the appender.
func (a *metricsAppender) SetOptions(_ *storage.AppendOptions) {
	// Not implemented for this component
}

// UpdateMetadata updates the metadata for a series.
func (a *metricsAppender) UpdateMetadata(_ storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	// Not implemented for this component
	return 0, nil
}
