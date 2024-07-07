package queue

import (
	"bytes"
	"context"
	"math"
	"sync"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/filequeue"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/types"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
)

// cborwriter is the primary class for serializing and deserializing metrics.
type cborwriter struct {
	mut          sync.Mutex
	fq           filequeue.MetricQueue
	totalSignals int64
	// estimatedSize tries to track how large the data size is.
	estimatedSize int64
	// checkpointSize is when to write a new entry to the queue.
	checkpointSize int64
	root           *cborroot
	bb             []byte
	// stringToIntMapping is used to map strings to an integer in the cborroot.
	stringToIntMapping map[string]int
	index              int
	l                  log.Logger
	flushTime          time.Duration
	metricGauge        prometheus.Gauge
	bytesWrittenGauge  prometheus.Gauge
	stop               *atomic.Bool
	em                 cbor.EncMode
}

// newCBORWrite creates a new parquetwriter.
func newCBORWrite(fq filequeue.MetricQueue, checkPointSize int64, flushTime time.Duration, l log.Logger, r prometheus.Registerer) *cborwriter {
	encOptions := cbor.CoreDetEncOptions()
	em, err := encOptions.EncMode()
	if err != nil {
		return nil
	}
	pw := &cborwriter{
		fq:             fq,
		checkpointSize: checkPointSize,
		estimatedSize:  0,
		totalSignals:   0,
		root: &cborroot{
			Strings:    make([]string, 0),
			TimeStamps: make(map[int64]*cbormetrictype),
		},
		l:                  l,
		flushTime:          flushTime,
		stringToIntMapping: make(map[string]int),
		bb:                 make([]byte, 0),
		metricGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alloy_queue_samples_to_wal_total",
			Help: "Number of samples written to the wal directory",
			ConstLabels: map[string]string{
				"name": fq.Name(),
			},
		}),
		bytesWrittenGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alloy_queue_bytes_written_total",
			Help: "Number of bytes written to the wal directory",
			ConstLabels: map[string]string{
				"name": fq.Name(),
			},
		}),
		stop: atomic.NewBool(false),
		em:   em,
	}
	r.Register(pw.metricGauge)
	r.Register(pw.bytesWrittenGauge)

	return pw
}

func (pw *cborwriter) SetCheckpointSize(checkpointSize int64) {
	pw.mut.Lock()
	defer pw.mut.Unlock()

	pw.checkpointSize = checkpointSize
}

func (pw *cborwriter) SetFlushTime(flushTime time.Duration) {
	pw.mut.Lock()
	defer pw.mut.Unlock()

	pw.flushTime = flushTime
}

var metricPool = sync.Pool{
	New: func() interface{} {
		return &cborsmetric{}
	},
}

// AddMetric is used to add a metric to the internal metrics for use with serialization.
func (pw *cborwriter) AddMetric(lbls labels.Labels, exemplarLabls labels.Labels, ts int64, val float64, histo *histogram.Histogram, floatHisto *histogram.FloatHistogram, telemetryType types.SeriesType) error {
	pw.mut.Lock()
	defer pw.mut.Unlock()

	tsNode, found := pw.root.TimeStamps[ts]
	if !found {
		pw.root.TimeStamps[ts] = &cbormetrictype{
			Samples:        make([]*cborsmetric, 0),
			Exemplars:      make([]*cborsmetric, 0),
			Histogram:      make([]*cborsmetric, 0),
			FloatHistogram: make([]*cborsmetric, 0),
		}
		tsNode = pw.root.TimeStamps[ts]
	}
	pm := metricPool.Get().(*cborsmetric)
	pw.addEstimatedSize(8)
	pm.Value = val
	pw.addEstimatedSize(8)

	pm.Labels = pw.addLabels(lbls, pm.Labels)
	pm.ExemplarLabels = pw.addLabels(exemplarLabls, pm.ExemplarLabels)
	if telemetryType == types.Histogram && histo != nil {
		pm.Histogram = cborhistgram{}
		pm.Histogram.CounterResetHint = int32(histo.CounterResetHint)
		pm.Histogram.Schema = histo.Schema
		pm.Value = histo.Sum
		pm.Histogram.PositiveBuckets = histo.PositiveBuckets
		pm.Histogram.NegativeBuckets = histo.NegativeBuckets
		pm.Histogram.NegativeSpans = pw.histogramSpanToSpan(histo.NegativeSpans)
		pm.Histogram.PositiveSpans = pw.histogramSpanToSpan(histo.PositiveSpans)
		pm.Histogram.Count = histo.Count
		pm.Histogram.ZeroCount = histo.ZeroCount
		pm.Histogram.ZeroThreshold = histo.ZeroThreshold
	} else if telemetryType == types.FloatHistogram && floatHisto != nil {
		pm.Histogram.CounterResetHint = int32(floatHisto.CounterResetHint)
		pm.Histogram.Schema = floatHisto.Schema
		pm.Value = floatHisto.Sum
		pm.Histogram.ZeroThreshold = floatHisto.ZeroThreshold
		pm.Histogram.NegativeSpans = pw.histogramSpanToSpan(floatHisto.NegativeSpans)
		pm.Histogram.PositiveSpans = pw.histogramSpanToSpan(floatHisto.PositiveSpans)
		pm.Histogram.FloatCount = floatHisto.Count
		pm.Histogram.FloatZeroCount = floatHisto.ZeroCount
		pm.Histogram.FloatNegativeBuckets = floatHisto.NegativeBuckets
		pm.Histogram.FloatPositiveBuckets = floatHisto.PositiveBuckets
	}

	switch telemetryType {
	case types.Histogram:
		tsNode.Histogram = append(tsNode.Histogram, pm)
	case types.FloatHistogram:
		tsNode.FloatHistogram = append(tsNode.FloatHistogram, pm)
	case types.Sample:
		tsNode.Samples = append(tsNode.Samples, pm)
	case types.Exemplar:
		tsNode.Exemplars = append(tsNode.Exemplars, pm)
	}
	pw.totalSignals += 1
	pw.root.TimeStamps[ts] = tsNode

	// We need to checkpoint
	if pw.estimatedSize > pw.checkpointSize {
		level.Debug(pw.l).Log("msg", "triggering write due to size", "estimated_size", pw.estimatedSize, "checkpoint_size", pw.checkpointSize)
		return pw.write()
	}
	return nil
}

func (pw *cborwriter) write() error {
	if pw.totalSignals == 0 {
		return nil
	}
	bb, err := pw.serialize()
	if err != nil {
		return err
	}
	pw.bytesWrittenGauge.Add(float64(len(bb)))
	_, err = pw.fq.Add(bb)
	return err
}

func (pw *cborwriter) serialize() ([]byte, error) {
	// No matter the error we should reset everything.
	defer func() {
		level.Debug(pw.l).Log("msg", "clearing estimated size, total signals and buffer", "estimated_size", pw.estimatedSize, "total_signal", pw.totalSignals)
		pw.estimatedSize = 0
		pw.totalSignals = 0
		for _, ts := range pw.root.TimeStamps {
			returnSliceToPool(ts.Exemplars)
			returnSliceToPool(ts.Samples)
			returnSliceToPool(ts.FloatHistogram)
			returnSliceToPool(ts.Histogram)
		}
		pw.root = &cborroot{
			Strings:    make([]string, 0),
			TimeStamps: make(map[int64]*cbormetrictype),
		}
		clear(pw.stringToIntMapping)
		pw.index = 0
	}()
	// Create our reverse dictionary
	sortedDictionary := make(map[int]string)
	for k, n := range pw.stringToIntMapping {
		sortedDictionary[n] = k
	}
	pw.root.Strings = make([]string, len(sortedDictionary))
	for i := 0; i < len(sortedDictionary); i++ {
		pw.root.Strings[i] = sortedDictionary[i]
	}
	bb, err := pw.em.Marshal(pw.root)
	pw.bb = snappy.Encode(pw.bb, bb)
	pw.metricGauge.Add(float64(pw.totalSignals))
	return pw.bb, err
}

func returnSliceToPool(pms []*cborsmetric) {
	for _, pm := range pms {
		returnToPool(pm)
	}
}

func returnToPool(pm *cborsmetric) {
	pm.ExemplarLabels = pm.ExemplarLabels[:0]
	pm.Labels = pm.Labels[:0]
	pm.Histogram = cborhistgram{}
	pm.Value = 0
	metricPool.Put(pm)
}

func (pw *cborwriter) addEstimatedSize(size int64) {
	pw.estimatedSize += size
}

func (pw *cborwriter) histogramSpanToSpan(histogramSpans []histogram.Span) []span {
	spans := make([]span, len(histogramSpans))
	for i, hs := range histogramSpans {
		spans[i] = span{
			Offset: hs.Offset,
			Length: hs.Length,
		}
		pw.addEstimatedSize(4 + 4)
	}
	return spans
}

func (pw *cborwriter) addLabels(input labels.Labels, vals []label) []label {
	if cap(vals) < len(input) {
		vals = make([]label, len(input))
	} else {
		vals = vals[:len(input)]
	}

	for i, l := range input {
		id, found := pw.stringToIntMapping[l.Name]
		if !found {
			pw.stringToIntMapping[l.Name] = pw.index
			id = pw.index
			pw.addEstimatedSize(int64(len(l.Name)) + 4)
			pw.index++
		}
		vals[i].Name = id
		id, found = pw.stringToIntMapping[l.Value]
		if !found {
			pw.stringToIntMapping[l.Value] = pw.index
			id = pw.index
			pw.addEstimatedSize(int64(len(l.Name)) + 4)
			pw.index++
		}
		vals[i].Value = id
		pw.addEstimatedSize(8)
	}
	return vals
}

// StartTimer ensures that data is flushed to disk every :.
func (pw *cborwriter) StartTimer(ctx context.Context) {
	t := time.NewTicker(pw.flushTime)
	for {
		if pw.stop.Load() {
			return
		}
		select {
		case <-t.C:
			pw.mut.Lock()
			pw.write()
			t.Reset(pw.flushTime)
			pw.mut.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (pw *cborwriter) Stop() {
	pw.stop.Store(true)
}

func spanToHistogramSpan(spans []span) []histogram.Span {
	returnSpans := make([]histogram.Span, len(spans))
	for i, hs := range spans {
		returnSpans[i] = histogram.Span{
			Offset: hs.Offset,
			Length: hs.Length,
		}
	}
	return returnSpans
}

func valuesAndKeysToLabels(dst labels.Labels, values []label, dict []string) labels.Labels {
	if cap(dst) < len(values) {
		dst = make(labels.Labels, len(values))
	} else {
		dst = dst[:len(values)]
	}
	for i, k := range values {
		dst[i].Name = dict[k.Name]
		dst[i].Value = dict[k.Value]
	}
	return dst
}

// buffer pool to reduce GC
var buffers = sync.Pool{
	// New is called when a new instance is needed
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func Deserialize(buffer []byte, maxAgeSeconds int64) ([]*types.TimeSeries, error) {
	root := &cborroot{}

	out := buffers.Get().(*bytes.Buffer)
	defer func() {
		out.Reset()
		buffers.Put(out)
	}()
	outBB := out.Bytes()
	outBB, err := snappy.Decode(outBB, buffer)
	if err != nil {
		return nil, err
	}
	decOpt := cbor.DecOptions{
		MaxArrayElements: math.MaxInt32,
	}
	dec, err := decOpt.DecMode()
	if err != nil {
		return nil, err
	}
	err = dec.Unmarshal(outBB, root)
	if err != nil {
		return nil, err
	}

	timeSeriesArray := make([]*types.TimeSeries, 0)
	for tsVal, metrics := range root.TimeStamps {

		// TODO make this common func
		for _, pm := range metrics.Exemplars {
			ts := makeTS(tsVal, maxAgeSeconds, pm, types.Exemplar, root.Strings)
			if ts != nil {
				timeSeriesArray = append(timeSeriesArray, ts)
			}
		}
		for _, pm := range metrics.Samples {
			ts := makeTS(tsVal, maxAgeSeconds, pm, types.Sample, root.Strings)
			if ts != nil {
				timeSeriesArray = append(timeSeriesArray, ts)
			}
		}
		for _, pm := range metrics.Histogram {
			ts := makeTS(tsVal, maxAgeSeconds, pm, types.Histogram, root.Strings)
			if ts != nil {
				timeSeriesArray = append(timeSeriesArray, ts)
			}
		}
		for _, pm := range metrics.FloatHistogram {
			ts := makeTS(tsVal, maxAgeSeconds, pm, types.FloatHistogram, root.Strings)
			if ts != nil {
				timeSeriesArray = append(timeSeriesArray, ts)
			}
		}
	}
	return timeSeriesArray, nil
}

func makeTS(tsVal int64, maxAgeSeconds int64, pm *cborsmetric, metricType types.SeriesType, dict []string) *types.TimeSeries {
	if tsVal < time.Now().Unix()-maxAgeSeconds && metricType != types.Exemplar {
		// TODO add drop metric here.
		return nil
	}
	// TODO can we instead directly use prompb.TimeSeries instead of this intermediate?
	ts := types.TimeSeriesPool.Get().(*types.TimeSeries)
	ts.Type = metricType
	ts.Timestamp = tsVal
	ts.Value = pm.Value
	ts.SeriesLabels = valuesAndKeysToLabels(ts.SeriesLabels, pm.Labels, dict)
	ts.ExemplarLabels = valuesAndKeysToLabels(ts.ExemplarLabels, pm.ExemplarLabels, dict)

	if ts.Type == types.Histogram {
		h := &histogram.Histogram{}
		h.CounterResetHint = histogram.CounterResetHint(pm.Histogram.CounterResetHint)
		h.Schema = pm.Histogram.Schema
		h.ZeroThreshold = pm.Histogram.ZeroThreshold
		h.ZeroCount = pm.Histogram.ZeroCount
		h.Count = pm.Histogram.Count
		h.PositiveSpans = spanToHistogramSpan(pm.Histogram.PositiveSpans)
		h.PositiveBuckets = pm.Histogram.PositiveBuckets
		h.NegativeSpans = spanToHistogramSpan(pm.Histogram.NegativeSpans)
		h.NegativeBuckets = pm.Histogram.NegativeBuckets
		h.Sum = pm.Value
		ts.Histogram = h
	} else if ts.Type == types.FloatHistogram {
		h := &histogram.FloatHistogram{}
		h.CounterResetHint = histogram.CounterResetHint(pm.Histogram.CounterResetHint)
		h.Schema = pm.Histogram.Schema
		h.ZeroThreshold = pm.Histogram.ZeroThreshold
		h.ZeroCount = pm.Histogram.FloatZeroCount
		h.Count = pm.Histogram.FloatCount
		h.PositiveSpans = spanToHistogramSpan(pm.Histogram.PositiveSpans)
		h.PositiveBuckets = pm.Histogram.FloatPositiveBuckets
		h.NegativeSpans = spanToHistogramSpan(pm.Histogram.NegativeSpans)
		h.NegativeBuckets = pm.Histogram.FloatNegativeBuckets
		h.Sum = pm.Value
		ts.FloatHistogram = h
	}
	return ts
}

type cborsmetric struct {
	Value          float64      `cbor:"4,keyasint"`
	Labels         []label      `cbor:"5,keyasint"`
	ExemplarLabels []label      `cbor:"6,keyasint,omitempty"`
	Histogram      cborhistgram `cbor:"7,keyasint,omitempty"`
}

type cborhistgram struct {
	// Shared Histogram
	CounterResetHint int32 `cbor:"7,keyasint,omitempty"`
	Schema           int32 `cbor:"8,keyasint,omitempty"`

	// Histogram
	ZeroThreshold   float64 `cbor:"9,keyasint,omitempty"`
	ZeroCount       uint64  `cbor:"10,keyasint,omitempty"`
	Count           uint64  `cbor:"11,keyasint,omitempty"`
	PositiveBuckets []int64 `cbor:"12,keyasint,omitempty"`
	NegativeBuckets []int64 `cbor:"13,keyasint,omitempty"`
	PositiveSpans   []span  `cbor:"14,keyasint,omitempty"`
	NegativeSpans   []span  `cbor:"15,keyasint,omitempty"`

	// FloatHistogram Fields
	FloatZeroCount       float64   `cbor:"16,keyasint,omitempty"`
	FloatCount           float64   `cbor:"17,keyasint,omitempty"`
	FloatPositiveBuckets []float64 `cbor:"18,keyasint,omitempty"`
	FloatNegativeBuckets []float64 `cbor:"19,keyasint,omitemptys"`
}

type cbormetrictype struct {
	Samples        []*cborsmetric `cbor:"1,keyasint"`
	Exemplars      []*cborsmetric `cbor:"2,keyasint"`
	Histogram      []*cborsmetric `cbor:"3,keyasint"`
	FloatHistogram []*cborsmetric `cbor:"4,keyasint"`
}

type cborroot struct {
	Strings    []string
	TimeStamps map[int64]*cbormetrictype
}

type span struct {
	_      struct{} `cbor:",toarray"`
	Offset int32    `cbor:"1,keyasint"`
	// Length of the span.
	Length uint32 `cbor:"2,keyasint"`
}

type label struct {
	_     struct{} `cbor:",toarray"`
	Name  int      `cbor:"1"`
	Value int      `cbor:"2"`
}
