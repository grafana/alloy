package queue

import (
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

var _ storage.Appender = (*fanout)(nil)

type fanout struct {
	children []storage.Appender
}

func (f fanout) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.Append(ref, l, t, v)
		if err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (f fanout) Commit() error {
	for _, child := range f.children {
		err := child.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}

func (f fanout) Rollback() error {
	for _, child := range f.children {
		err := child.Rollback()
		if err != nil {
			return err
		}
	}
	return nil
}

func (f fanout) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.AppendExemplar(ref, l, e)
		if err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (f fanout) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.AppendHistogram(ref, l, t, h, fh)
		if err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (f fanout) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.UpdateMetadata(ref, l, m)
		if err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (f fanout) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.AppendSTZeroSample(ref, l, t, st)
		if err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (f fanout) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.AppendHistogramSTZeroSample(ref, l, t, st, h, fh)
		if err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (f fanout) SetOptions(opts *storage.AppendOptions) {
	for _, child := range f.children {
		child.SetOptions(opts)
	}
}
