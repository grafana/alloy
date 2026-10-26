package appenders

import (
	"errors"
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
	// deadRefThreshold marks the boundary of the current ref generation. Any incoming
	// ref below this value is from a previous generation and meaningless to this child;
	// it must be zeroed so the child allocates a fresh ref.
	deadRefThreshold storage.SeriesRef
}

func NewPassthrough(wrapping storage.Appender, deadRefThreshold storage.SeriesRef, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.Appender {
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
		p.samplesForwarded.Inc()
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

// passthroughV2 is the storage.AppenderV2 counterpart of passthrough.
type passthroughV2 struct {
	wrapping         storage.AppenderV2
	start            time.Time
	writeLatency     prometheus.Histogram
	samplesForwarded prometheus.Counter
	// deadRefThreshold has the same meaning as in passthrough.
	deadRefThreshold storage.SeriesRef
}

func NewPassthroughV2(wrapping storage.AppenderV2, deadRefThreshold storage.SeriesRef, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.AppenderV2 {
	return &passthroughV2{
		wrapping:         wrapping,
		deadRefThreshold: deadRefThreshold,
		writeLatency:     writeLatency,
		samplesForwarded: samplesForwarded,
	}
}

func (p *passthroughV2) Append(ref storage.SeriesRef, l labels.Labels, st, t int64, v float64, h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options) (storage.SeriesRef, error) {
	if p.start.IsZero() {
		p.start = time.Now()
	}
	if ref != 0 && ref < p.deadRefThreshold {
		ref = 0
	}

	ref, err := p.wrapping.Append(ref, l, st, t, v, h, fh, opts)

	// Only float samples are counted, to match the V1 passthrough which only
	// counts calls to Append.
	if h == nil && fh == nil && sampleAppended(err) {
		p.samplesForwarded.Inc()
	}

	return ref, err
}

func (p *passthroughV2) Commit() error {
	defer p.recordLatency()
	return p.wrapping.Commit()
}

func (p *passthroughV2) Rollback() error {
	defer p.recordLatency()
	return p.wrapping.Rollback()
}

func (p *passthroughV2) recordLatency() {
	if p.start.IsZero() {
		return
	}
	duration := time.Since(p.start)
	p.writeLatency.Observe(duration.Seconds())
}

// sampleAppended reports whether a storage.AppenderV2.Append call that
// returned err still appended its sample. A *storage.AppendPartialError means
// the sample was appended but some of its exemplars were not.
func sampleAppended(err error) bool {
	if err == nil {
		return true
	}
	var partialErr *storage.AppendPartialError
	return errors.As(err, &partialErr)
}
