// Package testappender exposes utilities to test code which writes to
// Prometheus storage.Appenders.
package testappender

import (
	"fmt"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"

	"github.com/grafana/alloy/internal/util/testappender/internal/dtobuilder"
)

// Appender implements storage.Appender. It keeps track of samples, metadata,
// and exemplars written to it.
//
// When Commit is called, the written data will be converted into a slice of
// *dto.MetricFamily, when can then be used for asserting against expectations
// in tests.
//
// The zero value of Appender is ready for use. Appender is only intended for
// test code, and is not optimized for production.
//
// Appender is not safe for concurrent use.
type Appender struct {
	// HideTimestamps, when true, will omit timestamps from results.
	HideTimestamps bool

	commitCalled, rollbackCalled bool

	samples    map[string]dtobuilder.Sample          // metric labels -> sample
	exemplars  map[string]dtobuilder.SeriesExemplar  // metric labels -> series exemplar
	metadata   map[string]metadata.Metadata          // metric family name -> metadata
	histograms map[string]dtobuilder.SeriesHistogram // metric labels - > series histogram

	families []*dto.MetricFamily
}

var _ storage.Appender = (*Appender)(nil)

func (app *Appender) init() {
	if app.samples == nil {
		app.samples = make(map[string]dtobuilder.Sample)
	}
	if app.exemplars == nil {
		app.exemplars = make(map[string]dtobuilder.SeriesExemplar)
	}
	if app.metadata == nil {
		app.metadata = make(map[string]metadata.Metadata)
	}
	if app.histograms == nil {
		app.histograms = make(map[string]dtobuilder.SeriesHistogram)
	}
}

// Append adds or updates a sample for a given metric, identified by labels. l
// must not be empty. If Append is called twice for the same metric, older
// samples are discarded.
//
// Upon calling Commit, a MetricFamily is created for each unique `__name__`
// label. If UpdateMetadata is not called for the named series, the series will
// be treated as untyped. The timestamp of the metric will be reported with the
// value denoted by t.
//
// The ref field is ignored, and Append always returns 0 for the resulting
// storage.SeriesRef.
func (app *Appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	if app.commitCalled || app.rollbackCalled {
		return 0, fmt.Errorf("appender is closed")
	}
	app.init()

	l = l.WithoutEmpty()
	if l.Len() == 0 {
		return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
	}
	if lbl, dup := l.HasDuplicateLabelNames(); dup {
		return 0, fmt.Errorf("label name %q is not unique: %w", lbl, tsdb.ErrInvalidSample)
	}

	app.samples[l.String()] = dtobuilder.Sample{
		Labels:         l,
		Timestamp:      t,
		Value:          v,
		PrintTimestamp: !app.HideTimestamps,
	}
	return 0, nil
}

// AppendExemplar adds an exemplar for a given metric, identified by labels. l
// must not be empty.
//
// Upon calling Commit, exemplars are injected into the resulting Metrics for
// any Counter or Histogram.
//
// The ref field is ignored, and AppendExemplar always returns 0 for the
// resulting storage.SeriesRef.
func (app *Appender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	if app.commitCalled || app.rollbackCalled {
		return 0, fmt.Errorf("appender is closed")
	}
	app.init()

	l = l.WithoutEmpty()
	if l.Len() == 0 {
		return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
	}
	if lbl, dup := l.HasDuplicateLabelNames(); dup {
		return 0, fmt.Errorf("label name %q is not unique: %w", lbl, tsdb.ErrInvalidSample)
	}

	app.exemplars[l.String()] = dtobuilder.SeriesExemplar{
		Labels:   l,
		Exemplar: e,
	}
	return 0, nil
}

// UpdateMetadata associates metadata for a given named metric. l must not be
// empty. Only the `__name__` label is used from the label set; other labels
// are ignored.
//
// Upon calling Commit, the metadata will be injected into the associated
// MetricFamily. If m represents a histogram, metrics suffixed with `_bucket`,
// `_sum`, and `_count` will be brought into the same MetricFamily.
func (app *Appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	if app.commitCalled || app.rollbackCalled {
		return 0, fmt.Errorf("appender is closed")
	}
	app.init()

	l = l.WithoutEmpty()
	if l.Len() == 0 {
		return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
	}
	if lbl, dup := l.HasDuplicateLabelNames(); dup {
		return 0, fmt.Errorf("label name %q is not unique: %w", lbl, tsdb.ErrInvalidSample)
	}

	// Metadata is associated with just the metric family, retrieved by the
	// __name__ label. All metrics for that same name always have the same
	// metadata.
	familyName := l.Get(model.MetricNameLabel)
	if familyName == "" {
		return 0, fmt.Errorf("__name__ label missing: %w", tsdb.ErrInvalidSample)
	}
	app.metadata[familyName] = m
	return 0, nil
}

// AppendHistogram implements storage.Appendable
func (app *Appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if app.commitCalled || app.rollbackCalled {
		return 0, fmt.Errorf("appender is closed")
	}
	app.init()

	l = l.WithoutEmpty()
	if l.Len() == 0 {
		return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
	}
	if lbl, dup := l.HasDuplicateLabelNames(); dup {
		return 0, fmt.Errorf("label name %q is not unique: %w", lbl, tsdb.ErrInvalidSample)
	}
	app.histograms[l.String()] = dtobuilder.SeriesHistogram{
		Labels:    l,
		Histogram: *h,
	}
	return 0, nil
}

// Commit commits pending samples, exemplars, and metadata, converting them
// into a slice of *dto.MetricsFamily. Call MetricFamilies to get the resulting
// data.
//
// After calling Commit, no other methods except MetricFamilies may be called.
func (app *Appender) Commit() error {
	if app.commitCalled || app.rollbackCalled {
		return fmt.Errorf("appender is closed")
	}

	app.commitCalled = true
	app.families = dtobuilder.Build(
		app.samples,
		app.exemplars,
		app.histograms,
		app.metadata,
	)
	return nil
}

// Rollback discards pending samples, exemplars, and metadata.
//
// After calling Rollback, no other methods may be called and the Appender must
// be discarded.
func (app *Appender) Rollback() error {
	if app.commitCalled || app.rollbackCalled {
		return fmt.Errorf("appender is closed")
	}

	app.rollbackCalled = true
	return nil
}

// AppendCTZeroSample implements storage.Appender.
func (app *Appender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t int64, ct int64) (storage.SeriesRef, error) {
	panic("this test appender does not yet implement AppendCTZeroSample")
}

// AppendHistogramCTZeroSample implements storage.Appender.
func (app *Appender) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t int64, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	panic("this test appender does not yet implement AppendHistogramCTZeroSample")
}

// SetOptions implements storage.Appender.
func (app *Appender) SetOptions(o *storage.AppendOptions) {
	panic("this test appender does not yet implement SetOptions")
}

// MetricFamilies returns the generated slice of *dto.MetricsFamily.
// MetricFamilies returns an error unless Commit was called.
//
// MetricFamilies always returns a non-nil slice. If no data was appended, the
// resulting slice has a length of zero.
func (app *Appender) MetricFamilies() ([]*dto.MetricFamily, error) {
	if !app.commitCalled {
		return nil, fmt.Errorf("MetricFamilies is not ready")
	} else if app.rollbackCalled {
		return nil, fmt.Errorf("appender is closed")
	}

	return app.families, nil
}
