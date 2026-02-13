package appenders

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

type passthrough struct {
	wrapping         storage.Appender
	start            time.Time
	writeLatency     prometheus.Histogram
	samplesForwarded prometheus.Counter
}

func NewPassthrough(wrapping storage.Appender, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.Appender {
	return &passthrough{
		wrapping:         wrapping,
		writeLatency:     writeLatency,
		samplesForwarded: samplesForwarded,
	}
}

func (p *passthrough) Commit() error {
	defer p.recordLatency()
	return p.wrapping.Commit()
}

func (p *passthrough) Rollback() error {
	defer p.recordLatency()
	return p.wrapping.Rollback()
}

func (p *passthrough) recordLatency() {
	if p.start.IsZero() {
		return
	}
	duration := time.Since(p.start)
	p.writeLatency.Observe(duration.Seconds())
}

func (p *passthrough) SetOptions(opts *storage.AppendOptions) {
	p.wrapping.SetOptions(opts)
}

func (p *passthrough) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}

	ref, err := p.wrapping.Append(ref, l, t, v)

	if err == nil {
		p.samplesForwarded.Inc()
	}

	return ref, err
}

func (p *passthrough) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendExemplar(ref, l, e)
}

func (p *passthrough) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendHistogram(ref, l, t, h, fh)
}

func (p *passthrough) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendHistogramCTZeroSample(ref, l, t, ct, h, fh)
}

func (p *passthrough) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.UpdateMetadata(ref, l, m)
}

func (p *passthrough) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendCTZeroSample(ref, l, t, ct)
}
