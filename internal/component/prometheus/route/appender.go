package route

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

// appender opens downstream transactions only when a series selects them.
// Each receiver-list entry gets its own transaction, including duplicate entries.
type appender struct {
	component  *Component
	ctx        context.Context
	cfg        *routingConfig
	children   []storage.Appender
	options    *storage.AppendOptions
	optionsSet bool
}

var _ storage.Appender = (*appender)(nil)

func (a *appender) child(i int) storage.Appender {
	if a.children[i] == nil {
		a.children[i] = a.cfg.receivers[i].Appender(a.ctx)
		if a.optionsSet {
			a.children[i].SetOptions(a.options)
		}
	}
	return a.children[i]
}

func (a *appender) SetOptions(opts *storage.AppendOptions) {
	// Retain a copy for destinations first used after SetOptions returns.
	a.options = nil
	if opts != nil {
		copy := *opts
		a.options = &copy
	}
	a.optionsSet = true
	for _, child := range a.children {
		if child != nil {
			child.SetOptions(a.options)
		}
	}
}

func (a *appender) route(ref storage.SeriesRef, l labels.Labels, sample bool, send func(storage.Appender, storage.SeriesRef) (storage.SeriesRef, error)) (storage.SeriesRef, error) {
	if a.component.exited.Load() {
		return 0, fmt.Errorf("%s has exited", a.component.opts.ID)
	}
	// With the label store enabled, the enclosing Fanout supplies global refs.
	// Otherwise, refs are local to each receiver: resolve downstream series by
	// labels and return only router-local refs to upstream callers.
	inputRef := ref
	if !a.component.useLabelStore {
		ref = 0
	}
	if sample {
		a.component.processed.Inc()
	}
	var (
		matched  bool
		hasRef   bool
		attempts int
		result   error
	)
	forward := func(receivers receiverRange) {
		for i := receivers.start; i < receivers.end; i++ {
			attempts++
			childRef, err := send(a.child(i), ref)
			if err != nil {
				result = multierror.Append(result, err)
			} else {
				hasRef = hasRef || childRef != 0
				if sample {
					a.component.forwarded.Inc()
				}
			}
		}
	}
	for _, route := range a.cfg.routes {
		if route.selector.Matches(l) {
			matched = true
			forward(route.receivers)
		}
	}
	if !matched {
		forward(a.cfg.defaultReceivers)
	}
	if sample && attempts == 0 {
		a.component.dropped.Inc()
	}
	if !a.component.useLabelStore && hasRef {
		// Prometheus scrape uses nonzero, distinct refs to track disappearing
		// series. Reuse our input ref when possible; never reuse a child's ref.
		if inputRef == 0 {
			inputRef = storage.SeriesRef(a.component.nextRef.Inc())
		}
		return inputRef, result
	}
	return ref, result
}

func (a *appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	return a.route(ref, l, true, func(child storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return child.Append(ref, l, t, v)
	})
}

func (a *appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.route(ref, l, true, func(child storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return child.AppendHistogram(ref, l, t, h, fh)
	})
}

func (a *appender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return a.route(ref, l, false, func(child storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return child.AppendExemplar(ref, l, e)
	})
}

func (a *appender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return a.route(ref, l, false, func(child storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return child.UpdateMetadata(ref, l, m)
	})
}

func (a *appender) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	return a.route(ref, l, false, func(child storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return child.AppendSTZeroSample(ref, l, t, st)
	})
}

func (a *appender) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.route(ref, l, false, func(child storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return child.AppendHistogramSTZeroSample(ref, l, t, st, h, fh)
	})
}

func (a *appender) Commit() error {
	var result error
	for _, child := range a.children {
		if child != nil {
			if err := child.Commit(); err != nil {
				result = multierror.Append(result, err)
			}
		}
	}
	return result
}

func (a *appender) Rollback() error {
	var result error
	for _, child := range a.children {
		if child != nil {
			if err := child.Rollback(); err != nil {
				result = multierror.Append(result, err)
			}
		}
	}
	return result
}
