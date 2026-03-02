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
	samplesForwarded *prometheus.CounterVec
	// deadRefThreshold marks the boundary of the current ref generation. Any incoming
	// ref below this value is from a previous generation and meaningless to this child;
	// it must be zeroed so the child allocates a fresh ref.
	deadRefThreshold storage.SeriesRef
}

func NewPassthrough(wrapping storage.Appender, deadRefThreshold storage.SeriesRef, writeLatency prometheus.Histogram, samplesForwarded *prometheus.CounterVec) storage.Appender {
	if samplesForwarded != nil {
		// Initialize the counter so it appears at zero before any samples are forwarded.
		samplesForwarded.With(prometheus.Labels{"destination": ""})
	}
	return &passthrough{
		wrapping:         wrapping,
		deadRefThreshold: deadRefThreshold,
		writeLatency:     writeLatency,
		samplesForwarded: samplesForwarded,
	}
}

// sanitizeRef zeros ref if it is from a previous generation.
func (p *passthrough) sanitizeRef(ref storage.SeriesRef) storage.SeriesRef {
	if ref != 0 && ref < p.deadRefThreshold {
		return 0
	}
	return ref
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

	ref, err := p.wrapping.Append(p.sanitizeRef(ref), l, t, v)

	if err == nil {
		p.samplesForwarded.With(prometheus.Labels{"destination": ""}).Inc()
	}

	return ref, err
}

func (p *passthrough) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendExemplar(p.sanitizeRef(ref), l, e)
}

func (p *passthrough) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendHistogram(p.sanitizeRef(ref), l, t, h, fh)
}

func (p *passthrough) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendHistogramSTZeroSample(p.sanitizeRef(ref), l, t, st, h, fh)
}

func (p *passthrough) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.UpdateMetadata(p.sanitizeRef(ref), l, m)
}

func (p *passthrough) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	return p.wrapping.AppendSTZeroSample(p.sanitizeRef(ref), l, t, st)
}
