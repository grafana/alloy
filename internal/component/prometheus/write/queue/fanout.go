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

func (f fanout) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.AppendCTZeroSample(ref, l, t, ct)
		if err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (f fanout) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	for _, child := range f.children {
		_, err := child.AppendHistogramCTZeroSample(ref, l, t, ct, h, fh)
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
