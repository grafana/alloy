package appenders

import (
	"context"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

var (
	_ storage.AppendableV2 = AppendableV1AsV2{}
	_ storage.AppenderV2   = (*AppenderV1AsV2)(nil)
)

// AppendableV1AsV2 wraps a v1 storage.Appendable to satisfy storage.AppendableV2.
type AppendableV1AsV2 struct {
	Inner storage.Appendable
}

func (a AppendableV1AsV2) AppenderV2(ctx context.Context) storage.AppenderV2 {
	return &AppenderV1AsV2{Inner: a.Inner.Appender(ctx)}
}

// AppenderV1AsV2 wraps a v1 storage.Appender to satisfy storage.AppenderV2.
// It dispatches the unified Append call to the appropriate v1 methods.
type AppenderV1AsV2 struct {
	Inner storage.Appender
}

func (a *AppenderV1AsV2) Append(
	ref storage.SeriesRef,
	ls labels.Labels,
	st, t int64,
	v float64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
	opts storage.AppendV2Options,
) (storage.SeriesRef, error) {

	var err error
	switch {
	case fh != nil:
		ref, err = a.Inner.AppendHistogram(ref, ls, t, nil, fh)
	case h != nil:
		ref, err = a.Inner.AppendHistogram(ref, ls, t, h, nil)
	default:
		ref, err = a.Inner.Append(ref, ls, t, v)
	}
	if err != nil {
		return ref, err
	}

	// Per the AppenderV2 contract, the sample must be appended even if auxiliary
	// data (metadata, exemplars, ST zero) fails. Partial failures are collected
	// as AppendPartialError. See storage.AppenderV2.Append documentation:
	// https://github.com/prometheus/prometheus/blob/main/storage/interface_append.go
	var pErr *storage.AppendPartialError

	if st != 0 {
		switch {
		case fh != nil || h != nil:
			_, stErr := a.Inner.AppendHistogramSTZeroSample(ref, ls, t, st, h, fh)
			pErr, _ = pErr.Handle(stErr)
		default:
			_, stErr := a.Inner.AppendSTZeroSample(ref, ls, t, st)
			pErr, _ = pErr.Handle(stErr)
		}
	}

	if (opts.Metadata != metadata.Metadata{}) {
		_, mdErr := a.Inner.UpdateMetadata(ref, ls, opts.Metadata)
		pErr, _ = pErr.Handle(mdErr)
	}

	for _, e := range opts.Exemplars {
		if _, exemplarErr := a.Inner.AppendExemplar(ref, ls, e); exemplarErr != nil {
			if pErr == nil {
				pErr = &storage.AppendPartialError{}
			}
			pErr.ExemplarErrors = append(pErr.ExemplarErrors, exemplarErr)
		}
	}

	return ref, pErr.ToError()
}

func (a *AppenderV1AsV2) Commit() error {
	return a.Inner.Commit()
}

func (a *AppenderV1AsV2) Rollback() error {
	return a.Inner.Rollback()
}
