package batch

import (
	"math"
	"time"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

// appender is used to transfer from incoming samples to the PebbleDB.
type appender struct {
	parent         *Queue
	ttl            time.Duration
	l              *parquetwrite
	externalLabels labels.Labels
}

func newAppender(parent *Queue, ttl time.Duration, b *parquetwrite, externalLabels map[string]string) *appender {
	app := &appender{
		parent:         parent,
		ttl:            ttl,
		l:              b,
		externalLabels: labels.FromMap(externalLabels),
	}
	return app
}

// Append metric
func (a *appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}

	newLabels := labels.New(l...)
	newLabels = append(newLabels, a.externalLabels...)

	err := a.l.AddMetric(newLabels, nil, t, v, nil, nil, tSample)
	return ref, err
}

// Commit metrics to the DB
func (a *appender) Commit() (_ error) {
	return nil
}

// Rollback does nothing.
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

	newLabels := labels.New(l...)
	newLabels = append(newLabels, a.externalLabels...)
	err := a.l.AddMetric(newLabels, e.Labels, ts, e.Value, nil, nil, tSample)
	return ref, err
}

// AppendHistogram appends histogram
func (a *appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (_ storage.SeriesRef, _ error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}

	newLabels := labels.New(l...)
	newLabels = append(newLabels, a.externalLabels...)
	var err error
	if h != nil {
		err = a.l.AddMetric(newLabels, l, t, h.Sum, h, nil, tHistogram)
	} else if fh != nil {
		err = a.l.AddMetric(newLabels, l, t, h.Sum, nil, fh, tFloatHistogram)
	}
	return ref, err
}

// UpdateMetadata updates metadata.
func (a *appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (_ storage.SeriesRef, _ error) {
	// TODO allow metadata
	return 0, nil
}
