package queue

import (
	"math"
	"time"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

// appender is used to transfer from incoming samples to the underlying parquet interface.
type appender struct {
	parent *Queue
	ttl    time.Duration
	rs     *remotes
}

func newAppender(parent *Queue, ttl time.Duration, rs *remotes) *appender {
	app := &appender{
		parent: parent,
		ttl:    ttl,
		rs:     rs,
	}
	return app
}

// Append metric
func (a *appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}

	err := a.rs.AddMetric(l, nil, t, v, nil, nil, tSample)
	return ref, err
}

// Commit is a no op since we always write.
func (a *appender) Commit() (_ error) {
	return nil
}

// Rollback is a no op since we always write.
func (a *appender) Rollback() error {
	return nil
}

// AppendExemplar appends exemplar to cache.
func (a *appender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (_ storage.SeriesRef, _ error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if e.HasTs && e.Ts < endTime {
		return ref, nil
	}
	ts := int64(math.MaxInt64)
	if e.HasTs {
		ts = e.Ts
	}
	err := a.rs.AddMetric(l, e.Labels, ts, e.Value, nil, nil, tSample)
	return ref, err
}

// AppendHistogram appends histogram
func (a *appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (_ storage.SeriesRef, _ error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}

	var err error
	if h != nil {
		err = a.rs.AddMetric(l, l, t, h.Sum, h, nil, tHistogram)
	} else if fh != nil {
		err = a.rs.AddMetric(l, l, t, h.Sum, nil, fh, tFloatHistogram)
	}
	return ref, err
}

// UpdateMetadata updates metadata.
func (a *appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (_ storage.SeriesRef, _ error) {
	// TODO allow metadata
	return 0, nil
}
