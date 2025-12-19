package appenders

import (
	"context"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

type Noop struct {
}

func (n Noop) Appender(_ context.Context) storage.Appender {
	return n
}

func (n Noop) Append(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
	return ref, nil
}

func (n Noop) Commit() error {
	return nil
}

func (n Noop) Rollback() error {
	return nil
}

func (n Noop) SetOptions(_ *storage.AppendOptions) {
}

func (n Noop) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, _ exemplar.Exemplar) (storage.SeriesRef, error) {
	return ref, nil
}

func (n Noop) AppendHistogram(ref storage.SeriesRef, _ labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}

func (n Noop) AppendHistogramCTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}

func (n Noop) UpdateMetadata(ref storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	return ref, nil
}

func (n Noop) AppendCTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return ref, nil
}
