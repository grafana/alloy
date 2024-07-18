package cbor

import (
	"fmt"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/filequeue"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
)

type Raw struct {
	Hash  uint64 `cbor:"1,keyasint"`
	Bytes []byte `cbor:"2,keyasint"`
}

type SeriesGroup struct {
	Series   []*Raw `cbor:"1,keyasint"`
	Metadata []*Raw `cbor:"2,keyasint"`
}

type Serializer struct {
	mut           sync.RWMutex
	maxSizeBytes  int
	flushDuration time.Duration
	queue         filequeue.Queue
	group         *SeriesGroup
	lastFlush     time.Time
	bytesInGroup  uint32
}

func (s *Serializer) Append(l labels.Labels, t int64, v float64) error {
	ts := tsPool.Get().(*prompb.TimeSeries)
	defer returnTSToPool(ts)
	for _, l := range l {
		ts.Labels = append(ts.Labels, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	ts.Samples = append(ts.Samples, prompb.Sample{
		Value:     v,
		Timestamp: t,
	})
	data, err := ts.Marshal()
	if err != nil {
		return err
	}
	hash := l.Hash()
	return s.checkForPersist(hash, data)
}

// AppendExemplar appends exemplar to cache.
func (s *Serializer) AppendExemplar(l labels.Labels, e exemplar.Exemplar) error {
	ts := tsPool.Get().(*prompb.TimeSeries)
	defer returnTSToPool(ts)
	ex := prompb.Exemplar{}
	ex.Value = e.Value
	ex.Timestamp = e.Ts
	for _, l := range l {
		ex.Labels = append(ts.Labels, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	ts.Exemplars = append(ts.Exemplars, ex)
	data, err := ts.Marshal()
	if err != nil {
		return err
	}
	hash := l.Hash()
	return s.checkForPersist(hash, data)
}

// AppendHistogram appends histogram
func (s *Serializer) AppendHistogram(l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) error {
	ts := tsPool.Get().(*prompb.TimeSeries)
	defer returnTSToPool(ts)
	for _, l := range l {
		ts.Labels = append(ts.Labels, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	if h != nil {
		ts.Histograms = append(ts.Histograms, remote.HistogramToHistogramProto(t, h))
	} else {
		ts.Histograms = append(ts.Histograms, remote.FloatHistogramToHistogramProto(t, fh))
	}
	data, err := ts.Marshal()
	if err != nil {
		return err
	}
	hash := l.Hash()
	return s.checkForPersist(hash, data)
}

// UpdateMetadata updates metadata.
func (s *Serializer) UpdateMetadata(l labels.Labels, m metadata.Metadata) error {
	var name string

	for _, lbl := range l {
		if lbl.Name == "__name__" {
			name = lbl.Name
			break
		}
	}
	if name == "" {
		return fmt.Errorf("unable to find name for metadata")
	}
	md := prompb.MetricMetadata{
		Type: prompb.MetricMetadata_MetricType(prompb.MetricMetadata_MetricType_value[string(m.Type)]),
		Help: m.Help,
		Unit: m.Unit,
	}
	md.MetricFamilyName = name
	data, err := md.Marshal()
	if err != nil {
		return err
	}
	hash := l.Hash()
	return s.checkForPersist(hash, data)
}

func (s *Serializer) checkForPersist(hash uint64, data []byte) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.group.Series = append(s.group.Series, &Raw{
		Hash:  hash,
		Bytes: data,
	})
	s.bytesInGroup = +uint32(len(data)) + 4
	store := func() error {
		buffer, err := cbor.Marshal(s.group)
		if err != nil {
			// Something went wrong with serializing the whole group so lets drop it.
			s.group = &SeriesGroup{
				Series: make([]*Raw, 0),
			}
			return err
		}
		s.queue.Add(buffer)
		return nil
	}
	if s.bytesInGroup > uint32(s.maxSizeBytes) {
		return store()
	}
	if time.Now().Sub(s.lastFlush) > s.flushDuration {
		return store()
	}
	return nil
}

var tsPool = sync.Pool{
	New: func() any {
		return &prompb.TimeSeries{}
	},
}

func returnTSToPool(ts *prompb.TimeSeries) {
	ts.Histograms = ts.Histograms[:0]
	ts.Exemplars = ts.Exemplars[:0]
	ts.Samples = ts.Samples[:0]
	ts.Labels = ts.Labels[:0]
	tsPool.Put(ts)
}
