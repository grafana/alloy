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

var (
	_ storage.Appendable   = Noop{}
	_ storage.AppendableV2 = Noop{}
)

func (n Noop) Appender(_ context.Context) storage.Appender {
	return n
}

// AppenderV2 satisfies the AppendableV2 interface.
//
// TODO(v2 migration step 2): implement AppenderV2 for real. It currently
// panics because nothing calls it in production yet; all active append
// paths still go through Appender (V1). See https://github.com/grafana/alloy/issues/6896
func (n Noop) AppenderV2(_ context.Context) storage.AppenderV2 {
	panic("AppenderV2 not yet implemented for Noop")
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

func (n Noop) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}

func (n Noop) UpdateMetadata(ref storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	return ref, nil
}

func (n Noop) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return ref, nil
}
