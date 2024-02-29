package autoscrape

import (
	"fmt"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

// failedAppender is used as the appender when an instance couldn't be found.
type failedAppender struct {
	instanceName string
}

var _ storage.Appender = (*failedAppender)(nil)

func (fa *failedAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	return 0, fmt.Errorf("no such instance %s", fa.instanceName)
}

func (fa *failedAppender) Commit() error {
	return fmt.Errorf("no such instance %s", fa.instanceName)
}

func (fa *failedAppender) Rollback() error {
	return fmt.Errorf("no such instance %s", fa.instanceName)
}

func (fa *failedAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, fmt.Errorf("no such instance %s", fa.instanceName)
}

func (fa *failedAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return 0, fmt.Errorf("no such instance %s", fa.instanceName)
}

func (fa *failedAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return 0, fmt.Errorf("no such instance %s", fa.instanceName)
}
