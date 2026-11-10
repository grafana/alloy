package preview

import (
	"math"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/appenders/adapter"
)

func TestAppender(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		for _, rollback := range []bool{false, true} {
			t.Run(version+map[bool]string{false: "/commit", true: "/rollback"}[rollback], func(t *testing.T) {
				c := newTestComponent(t, testArguments())
				app := c.AppenderV2(t.Context())
				if version == "v1" {
					app = adapter.AppenderV1AsV2(c.Appender(t.Context()))
				}
				ls := labels.FromStrings("__name__", "requests_total", "secret", "private")
				stale := math.Float64frombits(value.StaleNaN)
				for _, sample := range []struct {
					ls labels.Labels
					v  float64
					h  *histogram.Histogram
					fh *histogram.FloatHistogram
				}{
					{ls: ls, v: 42},
					{ls: ls, v: 43},
					{ls: ls, v: stale},
					{ls: labels.FromStrings("__name__", "nan"), v: math.NaN()},
					{ls: labels.FromStrings("__name__", "histogram"), h: &histogram.Histogram{Sum: 12}},
					{ls: labels.FromStrings("__name__", "float_histogram"), fh: &histogram.FloatHistogram{Sum: 13}},
					{ls: labels.FromStrings("__name__", "stale_histogram"), h: &histogram.Histogram{Sum: stale}},
					{ls: labels.FromStrings("__name__", "stale_float_histogram"), fh: &histogram.FloatHistogram{Sum: stale}},
				} {
					ref, err := app.Append(123, sample.ls, 1, 2, sample.v, sample.h, sample.fh, storage.AppendV2Options{
						Metadata:  metadata.Metadata{Type: model.MetricTypeCounter, Help: "private"},
						Exemplars: []exemplar.Exemplar{{Labels: labels.FromStrings("trace_id", "private")}},
					})
					require.NoError(t, err)
					require.Zero(t, ref)
				}
				require.Zero(t, c.stats.total.series, "uncommitted observations must not appear in reports")
				if rollback {
					require.NoError(t, app.Rollback())
					require.Zero(t, c.stats.total.series)
					require.Equal(t, [2]uint64{}, c.stats.total.samples)
				} else {
					require.NoError(t, app.Commit())
					require.Equal(t, uint64(4), c.stats.total.series)
					require.Equal(t, [2]uint64{3, 2}, c.stats.total.samples)
				}
				require.ErrorContains(t, app.Commit(), "closed")
			})
		}
	}
}
