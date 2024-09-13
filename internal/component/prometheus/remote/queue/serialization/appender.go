package serialization

import (
	"context"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

type appender struct {
	ctx    context.Context
	ttl    time.Duration
	s      types.Serializer
	logger log.Logger
}

func (a *appender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	// TODO @mattdurham figure out what to do here later. This mirrors what we do elsewhere.
	return ref, nil
}

// NewAppender returns an Appender that writes to a given serializer. NOTE the Appender returned writes
// data immediately and does not honor commit or rollback.
func NewAppender(ctx context.Context, ttl time.Duration, s types.Serializer, logger log.Logger) storage.Appender {
	app := &appender{
		ttl:    ttl,
		s:      s,
		logger: logger,
		ctx:    ctx,
	}
	return app
}

// Append metric
func (a *appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	// Check to see if the TTL has expired for this record.
	endTime := time.Now().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}
	ts := types.GetTimeSeriesFromPool()
	ts.Labels = l
	ts.TS = t
	ts.Value = v
	ts.Hash = l.Hash()
	err := a.s.SendSeries(a.ctx, ts)
	return ref, err
}

// Commit is a no op since we always write.
func (a *appender) Commit() (_ error) {
	return nil
}

// Rollback is a no op since we write all the data.
func (a *appender) Rollback() error {
	return nil
}

// AppendExemplar appends exemplar to cache. The passed in labels is unused, instead use the labels on the exemplar.
func (a *appender) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, e exemplar.Exemplar) (_ storage.SeriesRef, _ error) {
	endTime := time.Now().Unix() - int64(a.ttl.Seconds())
	if e.HasTs && e.Ts < endTime {
		return ref, nil
	}
	ts := types.GetTimeSeriesFromPool()
	ts.Hash = e.Labels.Hash()
	ts.TS = e.Ts
	ts.Labels = e.Labels
	ts.Hash = e.Labels.Hash()
	err := a.s.SendSeries(a.ctx, ts)
	return ref, err
}

// AppendHistogram appends histogram
func (a *appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (_ storage.SeriesRef, _ error) {
	endTime := time.Now().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}
	ts := types.GetTimeSeriesFromPool()
	ts.Labels = l
	ts.TS = t
	if h != nil {
		ts.FromHistogram(t, h)
	} else {
		ts.FromFloatHistogram(t, fh)
	}
	ts.Hash = l.Hash()
	err := a.s.SendSeries(a.ctx, ts)
	return ref, err
}

// UpdateMetadata updates metadata.
func (a *appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (_ storage.SeriesRef, _ error) {
	ts := types.GetTimeSeriesFromPool()
	// We are going to handle converting some strings to hopefully not reused label names. TimeSeriesBinary has a lot of work
	// to ensure its efficient it makes sense to encode metadata into it.
	combinedLabels := l.Copy()
	combinedLabels = append(combinedLabels, labels.Label{
		Name:  "__alloy_metadata_type__",
		Value: string(m.Type),
	})
	combinedLabels = append(combinedLabels, labels.Label{
		Name:  "__alloy_metadata_help__",
		Value: m.Help,
	})
	combinedLabels = append(combinedLabels, labels.Label{
		Name:  "__alloy_metadata_unit__",
		Value: m.Unit,
	})
	ts.Labels = combinedLabels
	err := a.s.SendMetadata(a.ctx, ts)
	return ref, err
}
