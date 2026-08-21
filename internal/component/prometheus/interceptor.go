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
	onAppendSTZeroSample func(ref storage.SeriesRef, l labels.Labels, t, st int64, next storage.Appender) (storage.SeriesRef, error)

	onAppendV2 func(ref storage.SeriesRef, ls labels.Labels, st, t int64, v float64, h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options, next storage.AppenderV2) (storage.SeriesRef, error)

	// next is the next appendable to pass in the chain.
	next storage.AppendableV2

	componentID string
}

var _ storage.Appendable = (*Interceptor)(nil)
var _ storage.AppendableV2 = (*Interceptor)(nil)

// NewInterceptor creates a new Interceptor. Options can be provided to
// NewInterceptor to install custom hooks for different methods.
func NewInterceptor(next storage.AppendableV2, opts ...InterceptorOption) *Interceptor {
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

// WithSTZeroSampleHook returns an InterceptorOption which hooks into calls to
// AppendSTZeroSample.
func WithSTZeroSampleHook(f func(ref storage.SeriesRef, l labels.Labels, t, st int64, next storage.Appender) (storage.SeriesRef, error)) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppendSTZeroSample = f
	}
}

// WithAppendV2Hook returns an InterceptorOption which hooks into calls to
// AppenderV2's Append.
func WithAppendV2Hook(f func(ref storage.SeriesRef, ls labels.Labels, st, t int64, v float64, h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options, next storage.AppenderV2) (storage.SeriesRef, error)) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppendV2 = f
	}
}

// WithComponentID returns an InterceptorOptions which is used to set the componentID of the Interceptor.
// This is useful for debugging
func WithComponentID(id string) InterceptorOption {
	return func(i *Interceptor) {
		i.componentID = id
	}
}

// Appender satisfies the Appendable interface. Since next is AppendableV2, we
// type-assert it to storage.Appendable to obtain a v1 Appender. If the
// assertion fails the returned interceptappender has a nil child.
func (i *Interceptor) Appender(ctx context.Context) storage.Appender {
	app := &interceptappender{
		interceptor: i,
	}
	if v1, ok := i.next.(storage.Appendable); ok && v1 != nil {
		app.child = v1.Appender(ctx)
	}
	return app
}

// AppenderV2 satisfies the AppendableV2 interface.
func (i *Interceptor) AppenderV2(ctx context.Context) storage.AppenderV2 {
	app := &interceptappenderV2{
		interceptor: i,
	}
	if i.next != nil {
		app.child = i.next.AppenderV2(ctx)
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

func (a *interceptappender) AppendSTZeroSample(
	ref storage.SeriesRef,
	l labels.Labels,
	t, st int64,
) (storage.SeriesRef, error) {

	if a.interceptor.onAppendSTZeroSample != nil {
		return a.interceptor.onAppendSTZeroSample(ref, l, t, st, a.child)
	}
	if a.child == nil {
		return 0, nil
	}
	return a.child.AppendSTZeroSample(ref, l, t, st)
}

func (a *interceptappender) AppendHistogramSTZeroSample(
	ref storage.SeriesRef,
	l labels.Labels,
	t, st int64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
) (storage.SeriesRef, error) {

	if a.child == nil {
		return 0, nil
	}
	return a.child.AppendHistogramSTZeroSample(ref, l, t, st, h, fh)
}

// interceptappenderV2 implements storage.AppenderV2 and delegates to the
// Interceptor's onAppendV2 hook when set.
type interceptappenderV2 struct {
	interceptor *Interceptor
	child       storage.AppenderV2
}

var _ storage.AppenderV2 = (*interceptappenderV2)(nil)

// Append satisfies the AppenderV2 interface.
func (a *interceptappenderV2) Append(ref storage.SeriesRef, ls labels.Labels, st, t int64, v float64, h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options) (storage.SeriesRef, error) {
	if a.interceptor.onAppendV2 != nil {
		return a.interceptor.onAppendV2(ref, ls, st, t, v, h, fh, opts, a.child)
	}
	if a.child == nil {
		return 0, nil
	}
	return a.child.Append(ref, ls, st, t, v, h, fh, opts)
}

// Commit satisfies the AppenderV2 interface.
func (a *interceptappenderV2) Commit() error {
	if a.child == nil {
		return nil
	}
	return a.child.Commit()
}

// Rollback satisfies the AppenderV2 interface.
func (a *interceptappenderV2) Rollback() error {
	if a.child == nil {
		return nil
	}
	return a.child.Rollback()
}
