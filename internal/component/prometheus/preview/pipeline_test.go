package preview

import (
	"fmt"
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util/testappender"
)

// Exercise the same fanout and reference handling used by scrape and relabel,
// including cached input refs and both Prometheus appender APIs.
func TestPipelineReportsOnlyAggregates(t *testing.T) {
	for _, labelStoreEnabled := range []bool{false, true} {
		for _, v2 := range []bool{false, true} {
			t.Run(fmt.Sprintf("labelstore=%t/v2=%t", labelStoreEnabled, v2), func(t *testing.T) {
				sink := testappender.NewCollectingAppender()
				args := testArguments()
				args.ReportTo = []storage.AppendableV2{testappender.ConstantAppendable{Inner: sink}}
				c := newTestComponent(t, args)
				store := labelstore.New(logging.NewSlogNop(), prom.NewRegistry(), labelStoreEnabled)
				fanout := alloyprom.NewFanout([]storage.AppendableV2{c}, "prometheus.scrape.candidate", prom.NewRegistry(), store)
				t.Cleanup(fanout.Clear)
				ls := labels.FromStrings("__name__", "private_metric", "namespace", "candidate")
				var ref storage.SeriesRef
				var err error
				for i := range 3 {
					if v2 {
						app := fanout.AppenderV2(t.Context())
						ref, err = app.Append(ref, ls, 0, int64(i), 123456, nil, nil, storage.AppendV2Options{})
						require.NoError(t, err)
						require.NoError(t, app.Commit())
					} else {
						app := fanout.Appender(t.Context())
						ref, err = app.Append(ref, ls, int64(i), 123456)
						require.NoError(t, err)
						require.NoError(t, app.Commit())
					}
				}
				require.Equal(t, uint64(1), c.stats.total.series)
				require.Equal(t, uint64(3), c.stats.total.samples[0])
				require.Empty(t, sink.CollectedSamples())
				require.NoError(t, c.report(t.Context()))
				for _, sample := range sink.CollectedSamples() {
					require.NotContains(t, sample.Labels.String(), "private_metric")
					require.NotEqual(t, float64(123456), sample.Value)
				}
			})
		}
	}
}
