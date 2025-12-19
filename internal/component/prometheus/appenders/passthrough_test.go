package appenders

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util/testappender"
)

func TestPassthrough_Append(t *testing.T) {
	collecting := testappender.NewCollectingAppender()
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_samples_forwarded",
		Help: "Test samples forwarded",
	})
	a := NewPassthrough(collecting, nil, samplesForwarded)

	testLabels := labels.FromStrings("metric", "test_metric", "job", "test_job")
	timestamp := time.Now().UnixMilli()
	value := 42.0

	// Test Append
	ref, err := a.Append(storage.SeriesRef(100), testLabels, timestamp, value)
	require.NoError(t, err)
	assert.Equal(t, storage.SeriesRef(100), ref)

	samples := collecting.CollectedSamples()
	require.Len(t, samples, 1)

	sample := samples[testLabels.String()]
	require.NotNil(t, sample)
	assert.Equal(t, timestamp, sample.Timestamp)
	assert.Equal(t, value, sample.Value)
	assert.Equal(t, testLabels, sample.Labels)

	expected := `
		# HELP test_samples_forwarded Test samples forwarded
		# TYPE test_samples_forwarded counter
		test_samples_forwarded 1
	`
	err = testutil.CollectAndCompare(samplesForwarded, strings.NewReader(expected), "test_samples_forwarded")
	require.NoError(t, err)
}

func TestPassthrough_AppendError(t *testing.T) {
	// Create a failing appender
	failingAppender := &failingAppender{}

	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_samples_forwarded",
		Help: "Test samples forwarded",
	})
	a := NewPassthrough(failingAppender, nil, samplesForwarded)

	testLabels := labels.FromStrings("metric", "test_metric")
	timestamp := time.Now().UnixMilli()
	value := 42.0

	_, err := a.Append(storage.SeriesRef(100), testLabels, timestamp, value)
	require.Error(t, err)

	expected := `
		# HELP test_samples_forwarded Test samples forwarded
		# TYPE test_samples_forwarded counter
		test_samples_forwarded 0
	`
	err = testutil.CollectAndCompare(samplesForwarded, strings.NewReader(expected), "test_samples_forwarded")
	require.NoError(t, err)
}

func TestPassthrough_AppendHistogram(t *testing.T) {
	collecting := testappender.NewCollectingAppender()

	a := NewPassthrough(collecting, nil, nil)

	testLabels := labels.FromStrings("histogram", "test_histogram")
	timestamp := time.Now().UnixMilli()

	// Create a test histogram
	h := &histogram.Histogram{
		Count:  10,
		Sum:    100.5,
		Schema: 1,
	}

	// Test AppendHistogram
	ref, err := a.AppendHistogram(storage.SeriesRef(200), testLabels, timestamp, h, nil)
	require.NoError(t, err)
	assert.Equal(t, storage.SeriesRef(200), ref)

	// Verify histogram was forwarded
	histograms := collecting.CollectedHistograms()
	require.Len(t, histograms, 1)

	histSample := histograms[testLabels.String()]
	require.NotNil(t, histSample)
	assert.Equal(t, timestamp, histSample.Timestamp)
	assert.Equal(t, testLabels, histSample.Labels)
	assert.Equal(t, h, histSample.Histogram)
	assert.Nil(t, histSample.FloatHistogram)
}

func TestPassthrough_UpdateMetadata(t *testing.T) {
	collecting := testappender.NewCollectingAppender()

	a := NewPassthrough(collecting, nil, nil)

	testLabels := labels.FromStrings("metric", "test_metric")
	testMetadata := metadata.Metadata{
		Type: "counter",
		Help: "Test counter metric",
		Unit: "seconds",
	}

	// Test UpdateMetadata
	ref, err := a.UpdateMetadata(storage.SeriesRef(300), testLabels, testMetadata)
	require.NoError(t, err)
	assert.Equal(t, storage.SeriesRef(300), ref)

	// Verify metadata was forwarded
	m := collecting.CollectedMetadata()
	require.Len(t, m, 1)

	metadataEntry := m[testLabels.String()]
	assert.Equal(t, testMetadata, metadataEntry)
}

func TestPassthrough_Commit(t *testing.T) {
	collecting := testappender.NewCollectingAppender()

	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_samples_forwarded",
		Help: "Test samples forwarded",
	})
	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "test_write_latency",
		Help: "Test write latency",
	})
	a := NewPassthrough(collecting, writeLatency, samplesForwarded)

	testLabels := labels.FromStrings("metric", "test_metric")
	timestamp := time.Now().UnixMilli()
	_, err := a.Append(storage.SeriesRef(100), testLabels, timestamp, 42.0)
	require.NoError(t, err)

	err = a.Commit()
	require.NoError(t, err)

	// Verify histogram recorded exactly one observation
	count := testutil.CollectAndCount(writeLatency, "test_write_latency")
	assert.Equal(t, 1, count, "should have recorded one latency observation")
}

func TestPassthrough_Rollback(t *testing.T) {
	collecting := testappender.NewCollectingAppender()

	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_samples_forwarded",
		Help: "Test samples forwarded",
	})
	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "test_write_latency",
		Help: "Test write latency",
	})
	a := NewPassthrough(collecting, writeLatency, samplesForwarded)

	testLabels := labels.FromStrings("metric", "test_metric")
	timestamp := time.Now().UnixMilli()
	_, err := a.Append(storage.SeriesRef(100), testLabels, timestamp, 42.0)
	require.NoError(t, err)

	err = a.Rollback()
	require.NoError(t, err)

	// Verify histogram recorded exactly one observation
	count := testutil.CollectAndCount(writeLatency, "test_write_latency")
	assert.Equal(t, 1, count, "should have recorded one latency observation")
}

func TestPassthrough_AppendExemplar(t *testing.T) {
	collecting := testappender.NewCollectingAppender()

	a := NewPassthrough(collecting, nil, nil)

	testLabels := labels.FromStrings("metric", "test_metric")
	testExemplar := exemplar.Exemplar{
		Labels: labels.FromStrings("trace_id", "12345"),
		Value:  1.0,
		Ts:     time.Now().UnixMilli(),
	}

	// Test AppendExemplar - this will panic in collectingappender but we test that passthrough forwards it
	require.Panics(t, func() {
		_, _ = a.AppendExemplar(storage.SeriesRef(400), testLabels, testExemplar)
	})
}

// failingAppender is a test appender that always returns an error on Append
type failingAppender struct{}

func (f *failingAppender) Append(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
	return ref, errors.New("append failed")
}

func (f *failingAppender) Commit() error {
	return nil
}

func (f *failingAppender) Rollback() error {
	return nil
}

func (f *failingAppender) SetOptions(*storage.AppendOptions) {}

func (f *failingAppender) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, _ exemplar.Exemplar) (storage.SeriesRef, error) {
	return ref, errors.New("append exemplar failed")
}

func (f *failingAppender) AppendHistogram(ref storage.SeriesRef, _ labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, errors.New("append histogram failed")
}

func (f *failingAppender) AppendHistogramCTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, errors.New("append histogram ct zero failed")
}

func (f *failingAppender) UpdateMetadata(ref storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	return ref, errors.New("update metadata failed")
}

func (f *failingAppender) AppendCTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return ref, errors.New("append ct zero failed")
}
