package queue

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"time"

	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/cbor"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

type appender struct {
	parent   *Queue
	ttl      time.Duration
	s        *cbor.Serializer
	ts       *prompb.TimeSeries
	data     []*cbor.Raw
	metadata []*cbor.Raw
	logger   log.Logger
}

func newAppender(parent *Queue, ttl time.Duration, s *cbor.Serializer, logger log.Logger) *appender {
	app := &appender{
		parent:   parent,
		ttl:      ttl,
		s:        s,
		data:     make([]*cbor.Raw, 0),
		metadata: make([]*cbor.Raw, 0),
		logger:   logger,
		ts: &prompb.TimeSeries{
			Labels:     make([]prompb.Label, 0),
			Samples:    make([]prompb.Sample, 0),
			Exemplars:  make([]prompb.Exemplar, 0),
			Histograms: make([]prompb.Histogram, 0),
		},
	}
	return app
}

// Append metric
func (a *appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}
	for _, l := range l {
		a.ts.Labels = append(a.ts.Labels, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	a.ts.Samples = append(a.ts.Samples, prompb.Sample{
		Value:     v,
		Timestamp: t,
	})
	data, err := a.ts.Marshal()
	if err != nil {
		return ref, err
	}
	hash := l.Hash()
	a.data = append(a.data, &cbor.Raw{
		Hash:  hash,
		Ts:    t,
		Bytes: data,
	})
	a.resetTS()
	return ref, nil
}

// Commit is a no op since we always write.
func (a *appender) Commit() (_ error) {
	level.Debug(a.logger).Log("msg", "committing series to serializer", "len", len(a.data))
	err := a.s.Append(a.data)
	if err != nil {
		return err
	}
	level.Debug(a.logger).Log("msg", "committing metadata to serializer", "len", len(a.data))
	return a.s.AppendMetadata(a.metadata)
}

// Rollback is a no op.
func (a *appender) Rollback() error {
	return nil
}

// AppendExemplar appends exemplar to cache.
func (a *appender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (_ storage.SeriesRef, _ error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if e.HasTs && e.Ts < endTime {
		return ref, nil
	}
	ex := prompb.Exemplar{}
	ex.Value = e.Value
	ex.Timestamp = e.Ts
	for _, l := range l {
		ex.Labels = append(a.ts.Labels, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	a.ts.Exemplars = append(a.ts.Exemplars, ex)
	data, err := a.ts.Marshal()
	if err != nil {
		return ref, err
	}
	hash := l.Hash()
	a.data = append(a.data, &cbor.Raw{
		Hash:  hash,
		Ts:    ex.Timestamp,
		Bytes: data,
	})
	return ref, nil
}

// AppendHistogram appends histogram
func (a *appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (_ storage.SeriesRef, _ error) {
	endTime := time.Now().UTC().Unix() - int64(a.ttl.Seconds())
	if t < endTime {
		return ref, nil
	}
	for _, l := range l {
		a.ts.Labels = append(a.ts.Labels, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	if h != nil {
		a.ts.Histograms = append(a.ts.Histograms, remote.HistogramToHistogramProto(t, h))
	} else {
		a.ts.Histograms = append(a.ts.Histograms, remote.FloatHistogramToHistogramProto(t, fh))
	}
	data, err := a.ts.Marshal()
	if err != nil {
		return ref, err
	}
	hash := l.Hash()
	a.data = append(a.data, &cbor.Raw{
		Hash:  hash,
		Ts:    t,
		Bytes: data,
	})
	a.resetTS()
	return ref, nil
}

// UpdateMetadata updates metadata.
func (a *appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (_ storage.SeriesRef, _ error) {
	var name string
	for _, lbl := range l {
		if lbl.Name == "__name__" {
			name = lbl.Name
			break
		}
	}
	if name == "" {
		return ref, fmt.Errorf("unable to find name for metadata")
	}
	md := prompb.MetricMetadata{
		Type: prompb.MetricMetadata_MetricType(prompb.MetricMetadata_MetricType_value[string(m.Type)]),
		Help: m.Help,
		Unit: m.Unit,
	}
	md.MetricFamilyName = name
	data, err := md.Marshal()
	if err != nil {
		return ref, err
	}
	a.data = append(a.data, &cbor.Raw{
		Hash:  l.Hash(),
		Ts:    0,
		Bytes: data,
	})
	return ref, nil
}

func (a *appender) resetTS() {
	a.ts.Labels = a.ts.Labels[:0]
	a.ts.Samples = a.ts.Samples[:0]
	a.ts.Exemplars = a.ts.Exemplars[:0]
	a.ts.Histograms = a.ts.Histograms[:0]
}
