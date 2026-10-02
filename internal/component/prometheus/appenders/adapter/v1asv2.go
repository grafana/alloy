// Package adapter converts between Prometheus appender interface versions.
package adapter

import (
	"errors"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
)

// AppenderV1AsV2 adapts a storage.Appender (V1) to the storage.AppenderV2
// interface. It lets V2 callers write to storages that only implement V1
// natively, such as the WAL used by prometheus.remote_write.
//
// A single V2 Append is translated into the V1 calls a V1 caller such as the
// Prometheus scrape loop would make for the same sample, in the same order:
//
//  1. AppendSTZeroSample / AppendHistogramSTZeroSample if st is non-zero.
//     Errors are ignored, matching the best-effort ST handling in Prometheus.
//  2. Append / AppendHistogram for the sample itself, with the caller's ref.
//  3. AppendExemplar for each exemplar in opts, with the sample's ref.
//     Failures other than duplicates are returned as a
//     *storage.AppendPartialError.
//  4. UpdateMetadata if opts carries non-empty metadata, with the sample's
//     ref. Errors are ignored.
func AppenderV1AsV2(app storage.Appender) storage.AppenderV2 {
	return &v1AsV2{app: app}
}

type v1AsV2 struct {
	app storage.Appender

	// discardOutOfOrder mirrors the last value passed to app.SetOptions, so
	// that SetOptions is only called when AppendV2Options.RejectOutOfOrder
	// changes between appends.
	discardOutOfOrder bool
}

var _ storage.AppenderV2 = (*v1AsV2)(nil)

func (a *v1AsV2) Append(ref storage.SeriesRef, ls labels.Labels, st, t int64, v float64, h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options) (storage.SeriesRef, error) {
	if opts.RejectOutOfOrder != a.discardOutOfOrder {
		a.app.SetOptions(&storage.AppendOptions{DiscardOutOfOrder: opts.RejectOutOfOrder})
		a.discardOutOfOrder = opts.RejectOutOfOrder
	}

	isHistogram := h != nil || fh != nil

	// The ref returned by the ST append is deliberately not passed on to the
	// sample append. Some appenders don't translate refs for ST appends (for
	// example, the prometheus.remote_write Interceptor has no ST hook), so the
	// returned ref may not be one they accept as input.
	if st != 0 {
		if isHistogram {
			_, _ = a.app.AppendHistogramSTZeroSample(ref, ls, t, st, h, fh)
		} else {
			_, _ = a.app.AppendSTZeroSample(ref, ls, t, st)
		}
	}

	var err error
	if isHistogram {
		ref, err = a.app.AppendHistogram(ref, ls, t, h, fh)
	} else {
		ref, err = a.app.Append(ref, ls, t, v)
	}
	if err != nil {
		return ref, err
	}

	var partialErr *storage.AppendPartialError
	for _, e := range opts.Exemplars {
		if _, err := a.app.AppendExemplar(ref, ls, e); err != nil && !errors.Is(err, storage.ErrDuplicateExemplar) {
			if partialErr == nil {
				partialErr = &storage.AppendPartialError{}
			}
			partialErr.ExemplarErrors = append(partialErr.ExemplarErrors, err)
		}
	}

	if !opts.Metadata.IsEmpty() {
		_, _ = a.app.UpdateMetadata(ref, ls, opts.Metadata)
	}

	return ref, partialErr.ToError()
}

func (a *v1AsV2) Commit() error {
	return a.app.Commit()
}

func (a *v1AsV2) Rollback() error {
	return a.app.Rollback()
}
