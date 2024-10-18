package prometheus

import (
	"context"

	"github.com/prometheus/prometheus/model/metadata"
)

// NewInterceptor creates a new Interceptor storage.Appendable. Options can be
// provided to NewInterceptor to install custom hooks for different methods.
func NewInterceptor(next Appender, opts ...InterceptorOption) *Interceptor {
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
func WithAppendHook(f func(s []*Sample, next Appender, ctx context.Context) error) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppendSamples = f
	}
}

// WithMetadataHook returns an InterceptorOption which hooks into calls to
// UpdateMetadata.
func WithMetadataHook(f func(m []metadata.Metadata, next Appender, ctx context.Context) error) InterceptorOption {
	return func(i *Interceptor) {
		i.onUpdateMetadata = f
	}
}

// WithHistogramHook returns an InterceptorOption which hooks into calls to
// AppendHistogram.
func WithHistogramHook(f func(h []*Histogram, next Appender, ctx context.Context) error) InterceptorOption {
	return func(i *Interceptor) {
		i.onAppendHistograms = f
	}
}

type Interceptor struct {
	onAppendSamples    func(s []*Sample, next Appender, ctx context.Context) error
	onUpdateMetadata   func(m []metadata.Metadata, next Appender, ctx context.Context) error
	onAppendHistograms func(h []*Histogram, next Appender, ctx context.Context) error

	ctx context.Context

	// next is the next appendable to pass in the chain.
	next Appender
}

// Append satisfies the Appender interface.
func (a *Interceptor) Append(s []*Sample) error {
	if a.onAppendSamples != nil {
		return a.onAppendSamples(s, a.next, a.ctx)
	}
	if a.next == nil {
		return nil
	}
	return a.next.AppendSamples(s, a.ctx)
}

func (a *Interceptor) AppendHistograms(h []*Histogram) error {
	if a.onAppendHistograms != nil {
		return a.onAppendHistograms(h, a.next, a.ctx)
	}
	if a.next == nil {
		return nil
	}
	return a.next.AppendHistograms(h, a.ctx)
}
