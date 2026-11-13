package prometheus

import (
	"fmt"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/service/livedebugging"
)

// PublishAppendV2LiveDebugging publishes live debugging data for a single
// AppenderV2 Append call: one entry for the sample or histogram, one for the
// metadata if set, and one for each exemplar. Nothing is formatted or
// allocated if live debugging is not active for componentID.
func PublishAppendV2LiveDebugging(
	publisher livedebugging.DebugDataPublisher,
	componentID livedebugging.ComponentID,
	l labels.Labels,
	t int64,
	v float64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
	opts storage.AppendV2Options,
) {
	if !publisher.IsActive(componentID) {
		return
	}
	publish := func(f func() string) {
		publisher.PublishIfActive(livedebugging.NewData(componentID, livedebugging.PrometheusMetric, 1, f))
	}
	publish(func() string {
		switch {
		case h != nil:
			return fmt.Sprintf("histogram: ts=%d, labels=%s, value=%s", t, l, h.String())
		case fh != nil:
			return fmt.Sprintf("float_histogram: ts=%d, labels=%s, value=%s", t, l, fh.String())
		default:
			return fmt.Sprintf("sample: ts=%d, labels=%s, value=%f", t, l, v)
		}
	})
	if m := opts.Metadata; m != (metadata.Metadata{}) {
		publish(func() string {
			return fmt.Sprintf("metadata: labels=%s, type=%q, unit=%q, help=%q", l, m.Type, m.Unit, m.Help)
		})
	}
	for _, e := range opts.Exemplars {
		publish(func() string {
			return fmt.Sprintf("exemplar: ts=%d, labels=%s, exemplar_labels=%s, value=%f", e.Ts, l, e.Labels, e.Value)
		})
	}
}
