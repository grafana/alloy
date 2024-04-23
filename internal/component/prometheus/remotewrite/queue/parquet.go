package queue

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/parquet-go/parquet-go/compress/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/parquet-go/parquet-go"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
)

// parquetwrite is the primary class for serializing and deserializing metrics.
type parquetwrite struct {
	mut            sync.Mutex
	fq             metricQueue
	estimatedSize  int64
	totalSignals   int64
	checkpointSize int64
	pm             *parquetmetric
	writer         *parquet.GenericWriter[*parquetmetric]
	bb             *bytes.Buffer
	dictionary     map[string]int
	index          int
	l              log.Logger
	flushTime      time.Duration
	metricGauge    prometheus.Gauge
}

// newParquetWrite creates a new parquetwriter.
func newParquetWrite(fq metricQueue, checkPointSize int64, flushTime time.Duration, l log.Logger) *parquetwrite {
	pw := &parquetwrite{
		fq:             fq,
		checkpointSize: checkPointSize,
		estimatedSize:  0,
		totalSignals:   0,
		pm:             &parquetmetric{},
		l:              l,
		flushTime:      flushTime,
		dictionary:     make(map[string]int),
		bb:             &bytes.Buffer{},
		metricGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alloy_batch_metrics_to_wal",
			Help: "Number of metrics written to the wal directory",
			ConstLabels: map[string]string{
				"name": fq.Name(),
			},
		}),
	}
	pw.writer = parquet.NewGenericWriter[*parquetmetric](pw.bb, parquet.Compression(&snappy.Codec{}))

	return pw
}

func (pw *parquetwrite) SetCheckpointSize(checkpointSize int64) {
	pw.mut.Lock()
	defer pw.mut.Unlock()

	pw.checkpointSize = checkpointSize
}

func (pw *parquetwrite) SetFlushTime(flushTime time.Duration) {
	pw.mut.Lock()
	defer pw.mut.Unlock()

	pw.flushTime = flushTime
}

// AddMetric is used to add a metric to the internal metrics for use with serialization.
func (pw *parquetwrite) AddMetric(lbls labels.Labels, exemplarLabls labels.Labels, ts int64, val float64, histo *histogram.Histogram, floatHisto *histogram.FloatHistogram, telemetryType seriesType) error {
	pw.mut.Lock()
	defer pw.mut.Unlock()

	pm := &parquetmetric{}
	pm.Timestamp = ts
	pw.addEstimatedSize(8)
	pm.Value = val
	pw.addEstimatedSize(8)

	for _, l := range lbls {
		if l.Name == "__name__" {
			pm.Name = l.Value
			break
		}
	}

	pm.Type = telemetryType
	pm.Labels = pw.addLabels(lbls, pm.Labels)
	pm.ExemplarLabels = pw.addLabels(exemplarLabls, pm.ExemplarLabels)
	if telemetryType == tHistogram && histo != nil {
		pm.CounterResetHint = int32(histo.CounterResetHint)
		pm.Schema = histo.Schema
		pm.Value = histo.Sum
		pm.PositiveBuckets = histo.PositiveBuckets
		pm.NegativeBuckets = histo.NegativeBuckets
		pm.NegativeSpans = pw.histogramSpanToSpan(histo.NegativeSpans)
		pm.PositiveSpans = pw.histogramSpanToSpan(histo.PositiveSpans)
		pm.Count = histo.Count
		pm.ZeroCount = histo.ZeroCount
		pm.ZeroThreshold = histo.ZeroThreshold
	} else if telemetryType == tFloatHistogram && floatHisto != nil {
		pm.CounterResetHint = int32(floatHisto.CounterResetHint)
		pm.Schema = floatHisto.Schema
		pm.Value = floatHisto.Sum
		pm.ZeroThreshold = floatHisto.ZeroThreshold
		pm.NegativeSpans = pw.histogramSpanToSpan(floatHisto.NegativeSpans)
		pm.PositiveSpans = pw.histogramSpanToSpan(floatHisto.PositiveSpans)
		pm.FloatCount = floatHisto.Count
		pm.FloatZeroCount = floatHisto.ZeroCount
		pm.FloatNegativeBuckets = floatHisto.NegativeBuckets
		pm.FloatPositiveBuckets = floatHisto.PositiveBuckets
	}
	pw.writer.Write([]*parquetmetric{pm})
	pw.totalSignals += 1

	// We need to checkpoint
	if pw.estimatedSize > pw.checkpointSize {
		level.Debug(pw.l).Log("msg", "triggering write due to size", "estimated_size", pw.estimatedSize, "checkpoint_size", pw.checkpointSize)
		return pw.write()
	}
	return nil
}

func (pw *parquetwrite) write() error {
	if pw.totalSignals == 0 {
		return nil
	}
	bb, err := pw.serialize()
	if err != nil {
		return err
	}
	_, err = pw.fq.Add(bb)
	return err
}

func (pw *parquetwrite) serialize() ([]byte, error) {
	// No matter the error we should reset everything.
	defer func() {
		level.Debug(pw.l).Log("msg", "clearing estimated size, total signals and buffer", "estimated_size", pw.estimatedSize, "total_signal", pw.totalSignals)
		pw.estimatedSize = 0
		pw.totalSignals = 0
		clear(pw.dictionary)
		pw.bb.Reset()
		pw.writer.Reset(pw.bb)
		pw.index = 0
	}()
	tb := make([]byte, 8)
	pw.writer.Close()
	wrappedBB := &bytes.Buffer{}
	binary.BigEndian.PutUint64(tb, uint64(pw.bb.Len()))
	wrappedBB.Write(tb)
	wrappedBB.Write(pw.bb.Bytes())

	// Rest the buffer
	bb := &bytes.Buffer{}

	// Write the strings dictionary.
	strArr := make([]*stringMap, len(pw.dictionary))
	index := 0
	for k, v := range pw.dictionary {
		strArr[index] = &stringMap{
			ID:    v,
			Value: k,
		}
		index++
	}
	writeStrings := parquet.NewGenericWriter[*stringMap](bb, parquet.Compression(&snappy.Codec{}))
	_, err := writeStrings.Write(strArr)
	if err != nil {
		return nil, err
	}
	err = writeStrings.Close()
	if err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint64(tb, uint64(bb.Len()))
	wrappedBB.Write(tb)
	wrappedBB.Write(bb.Bytes())
	return wrappedBB.Bytes(), nil
}

func (pw *parquetwrite) addEstimatedSize(size int64) {
	pw.estimatedSize += size
}

func (pw *parquetwrite) histogramSpanToSpan(histogramSpans []histogram.Span) []span {
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

func (pw *parquetwrite) addLabels(input labels.Labels, vals []label) []label {
	if cap(vals) < len(input) {
		vals = make([]label, len(input))
	} else {
		vals = vals[:len(input)]
	}

	for i, l := range input {
		id, found := pw.dictionary[l.Name]
		if !found {
			pw.dictionary[l.Name] = pw.index
			id = pw.index
			pw.addEstimatedSize(int64(len(l.Name)) + 4)
			pw.index++
		}
		vals[i].Name = id
		id, found = pw.dictionary[l.Value]
		if !found {
			pw.dictionary[l.Value] = pw.index
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
func (pw *parquetwrite) StartTimer(ctx context.Context) {
	t := time.NewTicker(pw.flushTime)
	for {
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

func valuesAndKeysToLabels(values []label, dict map[int]string) labels.Labels {
	lbls := make([]labels.Label, len(values))
	for i, k := range values {
		lbls[i].Name = dict[k.Name]
		lbls[i].Value = dict[k.Value]
	}
	return lbls
}

func DeserializeParquet(buffer []byte, maxAgeSeconds int64) ([]TimeSeries, error) {
	length := binary.BigEndian.Uint64(buffer[0:8])
	metricBuffer := bytes.NewReader(buffer[8 : 8+length])
	rows, err := parquet.Read[*parquetmetric](metricBuffer, int64(metricBuffer.Len()))
	if err != nil {
		return nil, err
	}
	// We write the length but we can skip another 8 to get the starting point
	dictBuffer := bytes.NewReader(buffer[16+length:])
	stringRows, err := parquet.Read[*stringMap](dictBuffer, int64(dictBuffer.Len()))
	if err != nil {
		return nil, err
	}
	dict := make(map[int]string)
	for _, k := range stringRows {
		dict[k.ID] = k.Value
	}
	if err != nil {
		return nil, err
	}
	timeSeriesArray := make([]TimeSeries, 0)
	for _, pm := range rows {
		if pm.Timestamp < time.Now().Unix()-maxAgeSeconds && pm.Type != tExemplar {
			// TODO add drop metric here.
			continue
		}
		ts := TimeSeries{}
		ts.timestamp = pm.Timestamp
		ts.value = pm.Value
		ts.sType = pm.Type
		ts.seriesLabels = valuesAndKeysToLabels(pm.Labels, dict)
		ts.exemplarLabels = valuesAndKeysToLabels(pm.ExemplarLabels, dict)

		if ts.sType == tHistogram {
			h := &histogram.Histogram{}
			h.CounterResetHint = histogram.CounterResetHint(pm.CounterResetHint)
			h.Schema = pm.Schema
			h.ZeroThreshold = pm.ZeroThreshold
			h.ZeroCount = pm.ZeroCount
			h.Count = pm.Count
			h.PositiveSpans = spanToHistogramSpan(pm.PositiveSpans)
			h.PositiveBuckets = pm.PositiveBuckets
			h.NegativeSpans = spanToHistogramSpan(pm.NegativeSpans)
			h.NegativeBuckets = pm.NegativeBuckets
			h.Sum = pm.Value
			ts.histogram = h
		} else if ts.sType == tFloatHistogram {
			h := &histogram.FloatHistogram{}
			h.CounterResetHint = histogram.CounterResetHint(pm.CounterResetHint)
			h.Schema = pm.Schema
			h.ZeroThreshold = pm.ZeroThreshold
			h.ZeroCount = pm.FloatZeroCount
			h.Count = pm.FloatCount
			h.PositiveSpans = spanToHistogramSpan(pm.PositiveSpans)
			h.PositiveBuckets = pm.FloatPositiveBuckets
			h.NegativeSpans = spanToHistogramSpan(pm.NegativeSpans)
			h.NegativeBuckets = pm.FloatNegativeBuckets
			h.Sum = pm.Value
			ts.floatHistogram = h
		}
		timeSeriesArray = append(timeSeriesArray, ts)
	}
	return timeSeriesArray, nil
}

type parquetmetric struct {
	Name      string     `parquet:"name"`
	Type      seriesType `parquet:"telemetry,delta"`
	Timestamp int64      `parquet:"timestamp,delta"`
	Value     float64    `parquet:"value,split"`

	Labels         []label `parquet:"labels"`
	ExemplarLabels []label `parquet:"exemplar_labels,optional"`

	// Shared Histogram
	CounterResetHint int32 `parquet:"counter_reset_hint,optional,delta"`
	Schema           int32 `parquet:"schema,optional,delta"`

	// Histogram
	ZeroThreshold   float64 `parquet:"zero_threshold,optional,split"`
	ZeroCount       uint64  `parquet:"zero_count,optional,delta"`
	Count           uint64  `parquet:"count,optional,delta"`
	PositiveBuckets []int64 `parquet:"positive_buckets,optional"`
	NegativeBuckets []int64 `parquet:"negative_buckets,optional"`
	PositiveSpans   []span  `parquet:"positive_spans,optional"`
	NegativeSpans   []span  `parquet:"negative_spans,optional"`

	// FloatHistogram Fields
	FloatZeroCount       float64   `parquet:"float_zero_count,optional,split"`
	FloatCount           float64   `parquet:"float_count,optional,split"`
	FloatPositiveBuckets []float64 `parquet:"float_positive_buckets,optional"`
	FloatNegativeBuckets []float64 `parquet:"float_negative_buckets,optional"`
}

type span struct {
	Offset int32
	// Length of the span.
	Length uint32
}

type label struct {
	Name  int `parquet:"name,delta"`
	Value int `parquet:"value,delta"`
}

type stringMap struct {
	ID    int    `parquet:"id,delta"`
	Value string `parquet:"value"`
}
