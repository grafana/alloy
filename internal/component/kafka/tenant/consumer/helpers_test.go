package consumer

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	lokiclient "github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/alloytypes"
)

var testTopics = []string{"alloy-metrics", "alloy-logs", "alloy-traces", "alloy-profiles"}

const testRegistry = `
topics:
  metrics: alloy-metrics
  logs: alloy-logs
  traces: alloy-traces
  profiles: alloy-profiles
partitions: 4
tenants:
  tenant-a: 0
  tenant-b: 1
  tenant-c: 2
retired: [3]
`

var testTenants = map[string]int32{"tenant-a": 0, "tenant-b": 1, "tenant-c": 2}

// delivery is one record's worth of data as seen by a downstream component.
type delivery struct {
	Tenant string
	Format string
}

// recorder is a fake downstream for every signal. It records the tenant each
// delivery arrives with.
type recorder struct {
	mut sync.Mutex
	got []delivery

	logs loki.LogsReceiver
	stop chan struct{}

	// otelFn, if set, is called with the tenant of each OTLP delivery and its
	// error returned.
	otelFn func(tenant string) error
}

func newRecorder(t *testing.T) *recorder {
	r := &recorder{stop: make(chan struct{})}
	logs := loki.NewLogsReceiver()
	r.logs = logs
	go func() {
		for {
			select {
			case <-r.stop:
				return
			case e := <-logs.Chan():
				r.add(string(e.Labels[lokiclient.ReservedLabelTenantID]), kafkaclient.FormatLokiPush)
			}
		}
	}()
	t.Cleanup(func() { close(r.stop) })
	return r
}

func (r *recorder) add(tenant, format string) {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.got = append(r.got, delivery{Tenant: tenant, Format: format})
}

func (r *recorder) deliveries() []delivery {
	r.mut.Lock()
	defer r.mut.Unlock()
	return slices.Clone(r.got)
}

// args wires the recorder as the downstream of every signal.
func (r *recorder) args(brokers []string) Arguments {
	var args Arguments
	args.SetToDefault()
	args.Registry = alloytypes.OptionalSecret{Value: testRegistry}
	args.Client = kafkaclient.Arguments{Brokers: brokers}
	args.MinBackoff = time.Millisecond
	args.MaxBackoff = time.Millisecond
	args.SessionTimeout = 6 * time.Second
	args.MetricsForwardTo = []storage.Appendable{r}
	args.LogsForwardTo = []loki.LogsReceiver{r.logs}
	args.ProfilesForwardTo = []pyroscope.Appendable{pyroscope.AppendableIngestFunc(func(ctx context.Context, _ *pyroscope.IncomingProfile) error {
		tenant, _ := user.ExtractOrgID(ctx)
		r.add(tenant, kafkaclient.FormatPyroscopeIngest)
		return nil
	})}
	args.Output = &otelcol.ConsumerArguments{Metrics: []otelcol.Consumer{r}, Logs: []otelcol.Consumer{r}, Traces: []otelcol.Consumer{r}}
	return args
}

// OTLP consumer.

func (r *recorder) Capabilities() otelconsumer.Capabilities { return otelconsumer.Capabilities{} }

func (r *recorder) consumeOTLP(ctx context.Context) error {
	if r.otelFn != nil {
		if err := r.otelFn(otlpTenant(ctx)); err != nil {
			return err
		}
	}
	r.add(otlpTenant(ctx), kafkaclient.FormatOTLP)
	return nil
}

func otlpTenant(ctx context.Context) string {
	if v := client.FromContext(ctx).Metadata.Get(user.OrgIDHeaderName); len(v) > 0 {
		return v[0]
	}
	return ""
}

func (r *recorder) ConsumeMetrics(ctx context.Context, _ pmetric.Metrics) error {
	return r.consumeOTLP(ctx)
}
func (r *recorder) ConsumeLogs(ctx context.Context, _ plog.Logs) error { return r.consumeOTLP(ctx) }
func (r *recorder) ConsumeTraces(ctx context.Context, _ ptrace.Traces) error {
	return r.consumeOTLP(ctx)
}

// Prometheus appendable. One delivery is recorded per committed transaction.

func (r *recorder) Appender(ctx context.Context) storage.Appender {
	tenant, _ := user.ExtractOrgID(ctx)
	return &promAppender{r: r, tenant: tenant}
}

type promAppender struct {
	r       *recorder
	tenant  string
	samples int
}

func (a *promAppender) Append(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
	a.samples++
	return ref, nil
}
func (a *promAppender) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, _ exemplar.Exemplar) (storage.SeriesRef, error) {
	return ref, nil
}
func (a *promAppender) AppendHistogram(ref storage.SeriesRef, _ labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	a.samples++
	return ref, nil
}
func (a *promAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}
func (a *promAppender) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return ref, nil
}
func (a *promAppender) UpdateMetadata(ref storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	return ref, nil
}
func (a *promAppender) SetOptions(*storage.AppendOptions) {}
func (a *promAppender) Commit() error {
	if a.samples > 0 {
		a.r.add(a.tenant, kafkaclient.FormatPromRWv1)
	}
	return nil
}
func (a *promAppender) Rollback() error { return nil }

// Component helpers.

func testOptions(t *testing.T, id string) component.Options {
	return component.Options{
		ID:         id,
		Logger:     util.TestAlloyLogger(t).Slog(),
		Registerer: prometheus.NewRegistry(),
		GetServiceData: func(string) (any, error) {
			return labelstore.New(nil, prometheus.NewRegistry()), nil
		},
	}
}

// startConsumer runs a consumer until the returned stop func is called or the
// test ends.
func startConsumer(t *testing.T, id string, args Arguments, hook func(string, int32, bool)) (*Component, func()) {
	t.Helper()
	c, err := New(testOptions(t, id), args)
	require.NoError(t, err)
	c.processHook = hook

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(ctx) }()

	var once sync.Once
	stop := func() { once.Do(func() { cancel(); <-done }) }
	t.Cleanup(stop)
	return c, stop
}

func newProducer(t *testing.T, brokers []string) *kgo.Client {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RecordPartitioner(kgo.ManualPartitioner()))
	require.NoError(t, err)
	t.Cleanup(cl.Close)
	return cl
}

type testRecord struct {
	topic           string // Defaults to the signal's topic.
	partition       int32
	tenant          string
	signal          string
	format          string
	body            []byte
	contentType     string
	contentEncoding string
	url             string
}

func (tr testRecord) kgo() *kgo.Record {
	return &kgo.Record{
		Topic:     tr.topic,
		Partition: tr.partition,
		Key:       []byte(tr.tenant),
		Value:     tr.body,
		Headers: []kgo.RecordHeader{
			{Key: kafkaclient.HeaderTenantID, Value: []byte(tr.tenant)},
			{Key: kafkaclient.HeaderSignal, Value: []byte(tr.signal)},
			{Key: kafkaclient.HeaderFormat, Value: []byte(tr.format)},
			{Key: kafkaclient.HeaderContentType, Value: []byte(tr.contentType)},
			{Key: kafkaclient.HeaderContentEncoding, Value: []byte(tr.contentEncoding)},
			{Key: kafkaclient.HeaderURL, Value: []byte(tr.url)},
			{Key: kafkaclient.HeaderSchemaVersion, Value: []byte(kafkaclient.SchemaVersion)},
		},
	}
}

// topicFor returns the topic testRegistry maps signal to.
func topicFor(signal string) string { return "alloy-" + signal }

func produce(t *testing.T, cl *kgo.Client, recs ...testRecord) {
	t.Helper()
	krecs := make([]*kgo.Record, len(recs))
	for i, r := range recs {
		if r.topic == "" {
			r.topic = topicFor(r.signal)
		}
		krecs[i] = r.kgo()
	}
	require.NoError(t, cl.ProduceSync(t.Context(), krecs...).FirstErr())
}

// allSignals returns one record per ingest format for tenant.
func allSignals(t *testing.T, tenant string) []testRecord {
	p := testTenants[tenant]
	return []testRecord{
		{"", p, tenant, kafkaclient.SignalMetrics, kafkaclient.FormatPromRWv1, promRWBody(t), "application/x-protobuf", "snappy", "/api/v1/push"},
		{"", p, tenant, kafkaclient.SignalLogs, kafkaclient.FormatLokiPush, lokiBody(), "application/json", "", "/loki/api/v1/push"},
		{"", p, tenant, kafkaclient.SignalProfiles, kafkaclient.FormatPyroscopeIngest, []byte("pprof"), "application/octet-stream", "", "/ingest?name=app.cpu%7Benv%3Dprod%7D&from=1"},
		{"", p, tenant, kafkaclient.SignalTraces, kafkaclient.FormatOTLP, otlpTracesBody(t), "application/x-protobuf", "", "/v1/traces"},
	}
}

func promRWBody(t *testing.T) []byte {
	req := prompb.WriteRequest{Timeseries: []prompb.TimeSeries{{
		Labels:  []prompb.Label{{Name: "__name__", Value: "up"}, {Name: "job", Value: "test"}},
		Samples: []prompb.Sample{{Value: 1, Timestamp: time.Now().UnixMilli()}},
	}}}
	raw, err := req.Marshal()
	require.NoError(t, err)
	return snappy.Encode(nil, raw)
}

func lokiBody() []byte {
	return fmt.Appendf(nil, `{"streams":[{"stream":{"job":"test"},"values":[["%d","hello"]]}]}`, time.Now().UnixNano())
}

func otlpTracesBody(t *testing.T) []byte {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("op")
	b, err := ptraceotlp.NewExportRequestFromTraces(td).MarshalProto()
	require.NoError(t, err)
	return b
}
