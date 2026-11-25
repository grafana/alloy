// Package adapter converts between Prometheus appender interface versions.
package adapter

import (
	"errors"
	"fmt"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
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

	// lastMetadata holds the last metadata sent for each series in this
	// transaction. V2 callers attach the metadata to every sample of a series
	// and expect the storage to persist it only when it changed. V1 callers
	// call UpdateMetadata once per series.
	lastMetadata map[metadataKey]metadata.Metadata
}

// metadataKey identifies a series within a transaction. The series ref is
// exact. Appenders that don't return refs (ref 0) fall back to the labels, so
// two different series never share a key.
type metadataKey struct {
	ref  storage.SeriesRef
	lbls string
}

// metadataChanged reports whether m differs from the metadata last sent for
// the series (ref, ls) in this transaction, and records m.
func (a *v1AsV2) metadataChanged(ref storage.SeriesRef, ls labels.Labels, m metadata.Metadata) bool {
	key := metadataKey{ref: ref}
	if ref == 0 {
		key.lbls = ls.String()
	}
	if prev, ok := a.lastMetadata[key]; ok && prev == m {
		return false
	}
	if a.lastMetadata == nil {
		a.lastMetadata = make(map[metadataKey]metadata.Metadata)
	}
	a.lastMetadata[key] = m
	return true
}

var (
	_ storage.AppenderV2         = (*v1AsV2)(nil)
	_ storage.ExemplarAppenderV2 = (*v1AsV2)(nil)
)

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

	partialErr := a.appendExemplars(ref, ls, opts.Exemplars)

	if !opts.Metadata.IsEmpty() && a.metadataChanged(ref, ls, opts.Metadata) {
		_, _ = a.app.UpdateMetadata(ref, ls, opts.Metadata)
	}

	return ref, partialErr.ToError()
}

// AppendExemplars satisfies the storage.ExemplarAppenderV2 interface. It calls
// AppendExemplar of the V1 appender for each exemplar. Failures other than
// duplicates are returned as a *storage.AppendPartialError.
func (a *v1AsV2) AppendExemplars(ref storage.SeriesRef, ls labels.Labels, exemplars []exemplar.Exemplar) (storage.SeriesRef, error) {
	return ref, a.appendExemplars(ref, ls, exemplars).ToError()
}

func (a *v1AsV2) appendExemplars(ref storage.SeriesRef, ls labels.Labels, exemplars []exemplar.Exemplar) *storage.AppendPartialError {
	var partialErr *storage.AppendPartialError
	for i, e := range exemplars {
		if _, err := a.app.AppendExemplar(ref, ls, e); err != nil && !errors.Is(err, storage.ErrDuplicateExemplar) {
			if partialErr == nil {
				partialErr = &storage.AppendPartialError{}
			}
			partialErr.ExemplarErrors = append(partialErr.ExemplarErrors, storage.ExemplarError{Index: i, Err: err})
		}
	}
	return partialErr
}

// AppendExemplars forwards the exemplars to app. It returns an error if app
// does not implement storage.ExemplarAppenderV2.
func AppendExemplars(app storage.AppenderV2, ref storage.SeriesRef, ls labels.Labels, exemplars []exemplar.Exemplar) (storage.SeriesRef, error) {
	ea, ok := app.(storage.ExemplarAppenderV2)
	if !ok {
		return ref, fmt.Errorf("appender %T does not support appending exemplars", app)
	}
	return ea.AppendExemplars(ref, ls, exemplars)
}

func (a *v1AsV2) Commit() error {
	return a.app.Commit()
}

func (a *v1AsV2) Rollback() error {
	return a.app.Rollback()
}
