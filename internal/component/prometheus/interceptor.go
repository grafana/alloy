package prometheus

import (
	"context"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

// Interceptor is a storage.Appendable which invokes callback functions upon
// getting data. Interceptor should not be modified once created. All callback
// fields are optional.
type Interceptor struct {
	onAppend             func(ref storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error)
	onAppendExemplar     func(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error)
	onUpdateMetadata     func(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error)
	onAppendHistogram    func(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error)
	onAppendCTZeroSample func(ref storage.SeriesRef, l labels.Labels, t, ct int64, next storage.Appender) (storage.SeriesRef, error)

	// next is the next appendable to pass in the chain.
	next storage.Appendable

	componentID string
}

var _ storage.Appendable = (*Interceptor)(nil)

// NewInterceptor creates a new Interceptor storage.Appendable. Options can be
// provided to NewInterceptor to install custom hooks for different methods.
func NewInterceptor(next storage.Appendable, opts ...InterceptorOption) *Interceptor {
	i := &Interceptor{
		next: next,
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// InterceptorOption is an option argument passed to NewInterceptor.
type InterceptorOption func(*Interceptor)

// WithAppendHook returns an InterceptorOption which hooks into calls to
// Append.
func WithAppendHook(f func(ref storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error)) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppend = f
	}
}

// WithExemplarHook returns an InterceptorOption which hooks into calls to
// AppendExemplar.
func WithExemplarHook(f func(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error)) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppendExemplar = f
	}
}

// WithMetadataHook returns an InterceptorOption which hooks into calls to
// UpdateMetadata.
func WithMetadataHook(f func(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error)) InterceptorOption {
	return func(i *Interceptor) {
		i.onUpdateMetadata = f
	}
}

// WithHistogramHook returns an InterceptorOption which hooks into calls to
// AppendHistogram.
func WithHistogramHook(f func(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error)) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppendHistogram = f
	}
}

// WithCTZeroSampleHook returns an InterceptorOption which hooks into calls to
// AppendCTZeroSample.
func WithCTZeroSampleHook(f func(ref storage.SeriesRef, l labels.Labels, t, ct int64, next storage.Appender) (storage.SeriesRef, error)) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppendCTZeroSample = f
	}
}

// WithComponentID returns an InterceptorOptions which is used to set the componentID of the Interceptor.
// This is useful for debugging
func WithComponentID(id string) InterceptorOption {
	return func(i *Interceptor) {
		i.componentID = id
	}
}

// Appender satisfies the Appendable interface.
func (i *Interceptor) Appender(ctx context.Context) storage.Appender {
	app := &interceptappender{
		interceptor: i,
	}
	if i.next != nil {
		app.child = i.next.Appender(ctx)
	}
	return app
}

func (i *Interceptor) String() string {
	return i.componentID + ".receiver"
}

type interceptappender struct {
	interceptor *Interceptor
	child       storage.Appender
}

func (a *interceptappender) SetOptions(opts *storage.AppendOptions) {
	if a.child != nil {
		a.child.SetOptions(opts)
	}
}

var _ storage.Appender = (*interceptappender)(nil)

// Append satisfies the Appender interface.
func (a *interceptappender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	if a.interceptor.onAppend != nil {
		return a.interceptor.onAppend(ref, l, t, v, a.child)
	}
	if a.child == nil {
		return 0, nil
	}
	return a.child.Append(ref, l, t, v)
}

// Commit satisfies the Appender interface.
func (a *interceptappender) Commit() error {
	if a.child == nil {
		return nil
	}
	return a.child.Commit()
}

// Rollback satisfies the Appender interface.
func (a *interceptappender) Rollback() error {
	if a.child == nil {
		return nil
	}
	return a.child.Rollback()
}

// AppendExemplar satisfies the Appender interface.
func (a *interceptappender) AppendExemplar(
	ref storage.SeriesRef,
	l labels.Labels,
	e exemplar.Exemplar,
) (storage.SeriesRef, error) {

	if a.interceptor.onAppendExemplar != nil {
		return a.interceptor.onAppendExemplar(ref, l, e, a.child)
	}
	if a.child == nil {
		return 0, nil
	}
	return a.child.AppendExemplar(ref, l, e)
}

// UpdateMetadata satisfies the Appender interface.
func (a *interceptappender) UpdateMetadata(
	ref storage.SeriesRef,
	l labels.Labels,
	m metadata.Metadata,
) (storage.SeriesRef, error) {

	if a.interceptor.onUpdateMetadata != nil {
		return a.interceptor.onUpdateMetadata(ref, l, m, a.child)
	}
	if a.child == nil {
		return 0, nil
	}
	return a.child.UpdateMetadata(ref, l, m)
}

func (a *interceptappender) AppendHistogram(
	ref storage.SeriesRef,
	l labels.Labels,
	t int64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
) (storage.SeriesRef, error) {

	if a.interceptor.onAppendHistogram != nil {
		return a.interceptor.onAppendHistogram(ref, l, t, h, fh, a.child)
	}
	if a.child == nil {
		return 0, nil
	}
	return a.child.AppendHistogram(ref, l, t, h, fh)
}

func (a *interceptappender) AppendCTZeroSample(
	ref storage.SeriesRef,
	l labels.Labels,
	t, ct int64,
) (storage.SeriesRef, error) {

	if a.interceptor.onAppendCTZeroSample != nil {
		return a.interceptor.onAppendCTZeroSample(ref, l, t, ct, a.child)
	}
	if a.child == nil {
		return 0, nil
	}
	return a.child.AppendCTZeroSample(ref, l, t, ct)
}

func (a *interceptappender) AppendHistogramCTZeroSample(
	ref storage.SeriesRef,
	l labels.Labels,
	t, ct int64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
) (storage.SeriesRef, error) {

	if a.child == nil {
		return 0, nil
	}
	return a.child.AppendHistogramCTZeroSample(ref, l, t, ct, h, fh)
}
