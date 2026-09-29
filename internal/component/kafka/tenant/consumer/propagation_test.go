package consumer

import (
	"testing"
	"time"

	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
	lokiprocess "github.com/grafana/alloy/internal/component/loki/process"
	lokirelabel "github.com/grafana/alloy/internal/component/loki/relabel"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelbatch "github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	promrelabel "github.com/grafana/alloy/internal/component/prometheus/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	pyrorelabel "github.com/grafana/alloy/internal/component/pyroscope/relabel"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

// runIntermediate runs the named component and returns its exports.
func runIntermediate(t *testing.T, name string, args component.Arguments) component.Exports {
	t.Helper()
	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), name)
	require.NoError(t, err)
	go func() { _ = ctrl.Run(t.Context(), args) }()
	require.NoError(t, ctrl.WaitExports(5*time.Second))
	return ctrl.Exports()
}

func unmarshal[T any](t *testing.T, cfg string) T {
	t.Helper()
	var args T
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	return args
}

const relabelRule = `
rule {
  action       = "replace"
  target_label = "via"
  replacement  = "relabel"
}
`

// TestTenantPropagation checks that the tenant set by kafka.tenant_consumer
// still reaches the writer after passing through each intermediate component
// supported in consumer pipelines.
func TestTenantPropagation(t *testing.T) {
	tests := []struct {
		name   string
		format string
		// wire runs the intermediate component forwarding to rec and points
		// the consumer args at it.
		wire func(t *testing.T, rec *recorder, args *Arguments)
	}{
		{
			name:   "prometheus.relabel",
			format: kafkaclient.FormatPromRWv1,
			wire: func(t *testing.T, rec *recorder, args *Arguments) {
				a := unmarshal[promrelabel.Arguments](t, "forward_to = []\n"+relabelRule)
				a.ForwardTo = []storage.Appendable{rec}
				exports := runIntermediate(t, "prometheus.relabel", a).(promrelabel.Exports)
				args.MetricsForwardTo = []storage.Appendable{exports.Receiver}
			},
		},
		{
			name:   "pyroscope.relabel",
			format: kafkaclient.FormatPyroscopeIngest,
			wire: func(t *testing.T, rec *recorder, args *Arguments) {
				a := unmarshal[pyrorelabel.Arguments](t, "forward_to = []\n"+relabelRule)
				a.ForwardTo = args.ProfilesForwardTo
				exports := runIntermediate(t, "pyroscope.relabel", a).(pyrorelabel.Exports)
				args.ProfilesForwardTo = []pyroscope.Appendable{exports.Receiver}
			},
		},
		{
			name:   "loki.process",
			format: kafkaclient.FormatLokiPush,
			wire: func(t *testing.T, rec *recorder, args *Arguments) {
				a := unmarshal[lokiprocess.Arguments](t, `
					forward_to = []
					stage.static_labels {
						values = { via = "process" }
					}
				`)
				a.ForwardTo = []loki.LogsReceiver{rec.logs}
				exports := runIntermediate(t, "loki.process", a).(lokiprocess.Exports)
				args.LogsForwardTo = []loki.LogsReceiver{exports.Receiver}
			},
		},
		{
			name:   "loki.relabel",
			format: kafkaclient.FormatLokiPush,
			wire: func(t *testing.T, rec *recorder, args *Arguments) {
				a := unmarshal[lokirelabel.Arguments](t, "forward_to = []\n"+relabelRule)
				a.ForwardTo = []loki.LogsReceiver{rec.logs}
				exports := runIntermediate(t, "loki.relabel", a).(lokirelabel.Exports)
				args.LogsForwardTo = []loki.LogsReceiver{exports.Receiver}
			},
		},
		{
			name:   "otelcol.processor.batch",
			format: kafkaclient.FormatOTLP,
			wire: func(t *testing.T, rec *recorder, args *Arguments) {
				a := unmarshal[otelbatch.Arguments](t, `
					timeout       = "10ms"
					metadata_keys = ["X-Scope-OrgID"]
					output {}
				`)
				a.Output = &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{rec}}
				exports := runIntermediate(t, "otelcol.processor.batch", a).(otelcol.ConsumerExports)
				args.Output = &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{exports.Input}}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
			defer cluster.Close()

			rec := newRecorder(t)
			args := rec.args(cluster.ListenAddrs())
			tc.wire(t, rec, &args)
			startConsumer(t, "kafka.tenant_consumer.test", args, nil)

			cl := newProducer(t, cluster.ListenAddrs())
			for _, tenant := range []string{"tenant-a", "tenant-b"} {
				for _, r := range allSignals(t, tenant) {
					if r.format == tc.format {
						produce(t, cl, r)
					}
				}
			}

			require.Eventually(t, func() bool { return len(rec.deliveries()) == 2 }, 20*time.Second, 10*time.Millisecond)
			require.ElementsMatch(t, []delivery{{"tenant-a", tc.format}, {"tenant-b", tc.format}}, rec.deliveries())
		})
	}
}

// TestTenantPropagation_BatchWithoutMetadataKeys documents that
// otelcol.processor.batch drops the tenant unless metadata_keys includes
// X-Scope-OrgID.
func TestTenantPropagation_BatchWithoutMetadataKeys(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	rec := newRecorder(t)
	args := rec.args(cluster.ListenAddrs())
	a := unmarshal[otelbatch.Arguments](t, "timeout = \"10ms\"\noutput {}")
	a.Output = &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{rec}}
	exports := runIntermediate(t, "otelcol.processor.batch", a).(otelcol.ConsumerExports)
	args.Output = &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{exports.Input}}
	startConsumer(t, "kafka.tenant_consumer.test", args, nil)

	produce(t, newProducer(t, cluster.ListenAddrs()), allSignals(t, "tenant-a")[3])

	require.Eventually(t, func() bool { return len(rec.deliveries()) == 1 }, 20*time.Second, 10*time.Millisecond)
	require.Equal(t, []delivery{{"", kafkaclient.FormatOTLP}}, rec.deliveries())
}
