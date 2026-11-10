package preview

import (
	"context"
	"errors"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/prometheus/appenders/adapter"
)

func (c *Component) Appender(context.Context) storage.Appender {
	c.mu.Lock()
	defer c.mu.Unlock()
	return &appender{component: c, batch: newAggregation(c.args), generation: c.generation}
}

func (c *Component) AppenderV2(ctx context.Context) storage.AppenderV2 {
	return adapter.AppenderV1AsV2(c.Appender(ctx))
}

type appender struct {
	component  *Component
	batch      *aggregation
	generation uint64
}

var _ storage.Appender = (*appender)(nil)

func (a *appender) Append(_ storage.SeriesRef, ls labels.Labels, _ int64, v float64) (storage.SeriesRef, error) {
	if a.batch == nil {
		return 0, errors.New("preview appender is closed")
	}
	if !value.IsStaleNaN(v) {
		a.batch.observe(ls, false)
	}
	// Returning zero asks upstream to supply labels again, avoiding an
	// unbounded series-reference cache. No sample values are retained.
	return 0, nil
}

func (a *appender) AppendHistogram(_ storage.SeriesRef, ls labels.Labels, _ int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if a.batch == nil {
		return 0, errors.New("preview appender is closed")
	}
	if h != nil && !value.IsStaleNaN(h.Sum) || fh != nil && !value.IsStaleNaN(fh.Sum) {
		a.batch.observe(ls, true)
	}
	return 0, nil
}

func (a *appender) Commit() error {
	if a.batch == nil {
		return errors.New("preview appender is closed")
	}
	batch := a.batch
	a.batch = nil
	c := a.component
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return errors.New("preview component has stopped")
	}
	if a.generation != c.generation {
		return errors.New("preview measurement configuration changed before commit")
	}
	c.stats.merge(batch)
	return nil
}

func (a *appender) Rollback() error {
	a.batch = nil
	return nil
}

func (a *appender) SetOptions(*storage.AppendOptions) {}

func (a *appender) AppendExemplar(storage.SeriesRef, labels.Labels, exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *appender) UpdateMetadata(storage.SeriesRef, labels.Labels, metadata.Metadata) (storage.SeriesRef, error) {
	return 0, nil
}

// Synthetic start-time zero samples aren't source observations.
func (a *appender) AppendSTZeroSample(storage.SeriesRef, labels.Labels, int64, int64) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *appender) AppendHistogramSTZeroSample(storage.SeriesRef, labels.Labels, int64, int64, *histogram.Histogram, *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return 0, nil
}
