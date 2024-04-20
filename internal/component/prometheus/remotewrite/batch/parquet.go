package batch

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/parquet-go/parquet-go/compress/snappy"

	"github.com/parquet-go/parquet-go"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
)

type parquetmetric struct {
	Name           string
	Telemetry      TelemetryType
	Timestamp      int64
	Keys           []string
	Values         []string
	ExemplarKeys   []string
	ExemplarValues []string
	Value          float64

	// Shared Histogram
	CounterResetHint int32
	Schema           int32

	// Histogram
	ZeroThreshold   float64
	ZeroCount       uint64
	Count           uint64
	PositiveBuckets []int64
	NegativeBuckets []int64
	PositiveSpans   []span
	NegativeSpans   []span

	// FlostHistogram Fields
	FloatZeroCount       float64
	FloatCount           float64
	FloatPositiveBuckets []float64
	FloatNegativeBuckets []float64
}

type span struct {
	Offset int32
	// Length of the span.
	Length uint32
}

// parquetwrite is the primary class for serializing and deserializing metrics.
type parquetwrite struct {
	mut            sync.Mutex
	fq             *filequeue
	estimatedSize  int64
	totalSignals   int64
	checkpointSize int64
	bb             *bytes.Buffer
	buffer         *parquet.GenericWriter[*parquetmetric]
	pm             *parquetmetric
	l              log.Logger
	flushTime      time.Duration
}

// newParquetWrite creates a new parquetwriter.
func newParquetWrite(fq *filequeue, checkPointSize int64, flushTime time.Duration, l log.Logger) *parquetwrite {
	pw := &parquetwrite{
		fq:             fq,
		checkpointSize: checkPointSize,
		estimatedSize:  0,
		totalSignals:   0,
		bb:             &bytes.Buffer{},
		pm:             &parquetmetric{},
		l:              l,
		flushTime:      flushTime,
	}
	pw.buffer = parquet.NewGenericWriter[*parquetmetric](pw.bb, parquet.Compression(&snappy.Codec{}))
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
func (pw *parquetwrite) AddMetric(lbls labels.Labels, exemplarLabls labels.Labels, ts int64, val float64, histo *histogram.Histogram, floatHisto *histogram.FloatHistogram, telemetryType TelemetryType) error {
	pw.mut.Lock()
	defer pw.mut.Unlock()

	pw.pm.Timestamp = ts
	pw.addEstimatedSize(8)
	pw.pm.Value = val
	pw.addEstimatedSize(8)

	for _, l := range lbls {
		if l.Name == "__name__" {
			pw.pm.Name = l.Value
			break
		}
	}

	pw.pm.Telemetry = telemetryType
	pw.pm.Keys, pw.pm.Values = pw.addLabels(lbls, pw.pm.Keys, pw.pm.Values)
	pw.pm.ExemplarKeys, pw.pm.ExemplarValues = pw.addLabels(exemplarLabls, pw.pm.ExemplarKeys, pw.pm.ExemplarValues)
	if telemetryType == tHistogram && histo != nil {
		pw.pm.CounterResetHint = int32(histo.CounterResetHint)
		pw.pm.Schema = histo.Schema
		pw.pm.Value = histo.Sum
		pw.pm.PositiveBuckets = histo.PositiveBuckets
		pw.pm.NegativeBuckets = histo.NegativeBuckets
		pw.pm.NegativeSpans = pw.histogramSpanToSpan(histo.NegativeSpans)
		pw.pm.PositiveSpans = pw.histogramSpanToSpan(histo.PositiveSpans)
		pw.pm.Count = histo.Count
		pw.pm.ZeroCount = histo.ZeroCount
		pw.pm.ZeroThreshold = histo.ZeroThreshold
	} else if telemetryType == tFloatHistogram && floatHisto != nil {
		pw.pm.CounterResetHint = int32(floatHisto.CounterResetHint)
		pw.pm.Schema = floatHisto.Schema
		pw.pm.Value = floatHisto.Sum
		pw.pm.ZeroThreshold = floatHisto.ZeroThreshold
		pw.pm.NegativeSpans = pw.histogramSpanToSpan(floatHisto.NegativeSpans)
		pw.pm.PositiveSpans = pw.histogramSpanToSpan(floatHisto.PositiveSpans)
		pw.pm.FloatCount = floatHisto.Count
		pw.pm.FloatZeroCount = floatHisto.ZeroCount
		pw.pm.FloatNegativeBuckets = floatHisto.NegativeBuckets
		pw.pm.FloatPositiveBuckets = floatHisto.PositiveBuckets
	}

	pw.buffer.Write([]*parquetmetric{pw.pm})
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
	// We should always reset.
	defer pw.bb.Reset()
	err := pw.serialize()
	if err != nil {
		return err
	}
	_, err = pw.fq.AddCommited(pw.bb.Bytes())
	return err
}

func (pw *parquetwrite) serialize() error {
	// No matter the error we should reset everything.
	defer func() {
		level.Debug(pw.l).Log("msg", "clearing estimated size, total signals and buffer")
		pw.estimatedSize = 0
		pw.totalSignals = 0
		pw.buffer.Reset(pw.bb)
	}()
	err := pw.buffer.Close()
	if err != nil {
		return err
	}
	return nil
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

func (pw *parquetwrite) addLabels(input labels.Labels, keys, values []string) ([]string, []string) {
	if cap(values) < len(input) {
		values = make([]string, len(input))
		keys = make([]string, len(input))
	} else {
		values = values[:len(input)]
		keys = keys[:len(input)]
	}

	for i, l := range input {
		keys[i] = l.Name
		values[i] = l.Value
		pw.addEstimatedSize(int64(len(l.Name) + len(l.Value)))
	}
	return keys, values
}

// StartTimer ensures that data is flushed to disk every :.
func (pw *parquetwrite) StartTimer(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-t.C:
			pw.mut.Lock()
			pw.write()
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

func valuesAndKeysToLabels(keys, values []string) labels.Labels {
	lbls := make([]labels.Label, len(keys))
	for i, k := range keys {
		lbls[i].Name = k
		lbls[i].Value = values[i]
	}
	return lbls
}

func DeserializeParquet(buffer []byte, maxAgeSeconds int64) ([]*TimeSeries, error) {
	reader := bytes.NewReader(buffer)
	rows, err := parquet.Read[*parquetmetric](reader, int64(len(buffer)))
	if err != nil {
		return nil, err
	}
	timeSeriesArray := make([]*TimeSeries, 0)
	for _, pm := range rows {
		if pm.Timestamp < time.Now().Unix()-maxAgeSeconds && pm.Telemetry != tExemplar {
			// TODO add drop metric here.
			continue
		}
		ts := &TimeSeries{}
		ts.Timestamp = pm.Timestamp
		ts.Value = pm.Value
		ts.SeriesType = pm.Telemetry
		ts.SeriesLabels = valuesAndKeysToLabels(pm.Keys, pm.Values)
		ts.ExemplarLabels = valuesAndKeysToLabels(pm.ExemplarKeys, pm.ExemplarValues)

		if ts.SeriesType == tHistogram {
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
			ts.Histogram = h
		} else if ts.SeriesType == tFloatHistogram {
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
			ts.FloatHistogram = h
		}
		timeSeriesArray = append(timeSeriesArray, ts)
	}
	return timeSeriesArray, nil
}
