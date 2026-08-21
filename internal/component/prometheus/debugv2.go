package prometheus

import (
	"fmt"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/service/livedebugging"
)

// PublishV2DebugData publishes live debugging data for a v2 Append call.
func PublishV2DebugData(
	publisher livedebugging.DebugDataPublisher,
	componentID livedebugging.ComponentID,
	l labels.Labels,
	t int64, v float64,
	h *histogram.Histogram, fh *histogram.FloatHistogram,
	opts storage.AppendV2Options,
) {
	if !publisher.IsActive(componentID) {
		return
	}

	var data string
	switch {
	case fh != nil:
		data = fmt.Sprintf("float_histogram: ts=%d, labels=%s, value=%s", t, l, fh.String())
	case h != nil:
		data = fmt.Sprintf("histogram: ts=%d, labels=%s, value=%s", t, l, h.String())
	default:
		data = fmt.Sprintf("sample: ts=%d, labels=%s, value=%f", t, l, v)
	}
	publisher.PublishIfActive(livedebugging.NewData(
		componentID, livedebugging.PrometheusMetric, 1,
		func() string { return data },
	))

	if opts.Metadata != (metadata.Metadata{}) {
		publisher.PublishIfActive(livedebugging.NewData(
			componentID, livedebugging.PrometheusMetric, 1,
			func() string {
				return fmt.Sprintf("metadata: labels=%s, type=%q, unit=%q, help=%q", l, opts.Metadata.Type, opts.Metadata.Unit, opts.Metadata.Help)
			},
		))
	}

	for _, e := range opts.Exemplars {
		publisher.PublishIfActive(livedebugging.NewData(
			componentID, livedebugging.PrometheusMetric, 1,
			func() string {
				return fmt.Sprintf("exemplar: ts=%d, labels=%s, exemplar_labels=%s, value=%f", e.Ts, l, e.Labels, e.Value)
			},
		))
	}
}
