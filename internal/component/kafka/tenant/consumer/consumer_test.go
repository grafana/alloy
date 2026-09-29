package consumer

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
)

func TestConsumer_ForwardsAllFormatsWithTenant(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	rec := newRecorder(t)
	c, _ := startConsumer(t, "kafka.tenant_consumer.test", rec.args(cluster.ListenAddrs()), nil)

	cl := newProducer(t, cluster.ListenAddrs())
	produce(t, cl, allSignals(t, "tenant-a")...)
	produce(t, cl, allSignals(t, "tenant-b")...)

	require.Eventually(t, func() bool { return len(rec.deliveries()) == 8 }, 20*time.Second, 10*time.Millisecond)
	require.ElementsMatch(t, []delivery{
		{"tenant-a", kafkaclient.FormatPromRWv1},
		{"tenant-a", kafkaclient.FormatLokiPush},
		{"tenant-a", kafkaclient.FormatPyroscopeIngest},
		{"tenant-a", kafkaclient.FormatOTLP},
		{"tenant-b", kafkaclient.FormatPromRWv1},
		{"tenant-b", kafkaclient.FormatLokiPush},
		{"tenant-b", kafkaclient.FormatPyroscopeIngest},
		{"tenant-b", kafkaclient.FormatOTLP},
	}, rec.deliveries())
	require.Equal(t, component.HealthTypeHealthy, c.CurrentHealth().Health)
}

func TestConsumer_TenantGuardDropsAndContinues(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	rec := newRecorder(t)
	c, _ := startConsumer(t, "kafka.tenant_consumer.test", rec.args(cluster.ListenAddrs()), nil)

	cl := newProducer(t, cluster.ListenAddrs())
	mismatch := allSignals(t, "tenant-a")[3]
	mismatch.tenant = "tenant-b" // On tenant-a's partition.
	retired := allSignals(t, "tenant-a")[3]
	retired.partition = 3
	wrongTopic := allSignals(t, "tenant-a")[0] // A metrics record in the traces topic.
	wrongTopic.topic = topicFor(kafkaclient.SignalTraces)
	produce(t, cl, mismatch, retired, wrongTopic, allSignals(t, "tenant-a")[3])

	require.Eventually(t, func() bool { return len(rec.deliveries()) == 1 }, 20*time.Second, 10*time.Millisecond)
	require.Equal(t, []delivery{{"tenant-a", kafkaclient.FormatOTLP}}, rec.deliveries())
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.metrics.dropped.WithLabelValues("topic_mismatch", kafkaclient.FormatPromRWv1)) == 1
	}, 20*time.Second, 10*time.Millisecond)
	require.Equal(t, 1.0, testutil.ToFloat64(c.metrics.dropped.WithLabelValues("tenant_mismatch", kafkaclient.FormatOTLP)))
	require.Equal(t, 1.0, testutil.ToFloat64(c.metrics.dropped.WithLabelValues("unassigned_partition", kafkaclient.FormatOTLP)))
}

func TestConsumer_MalformedRecordsDoNotStall(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	rec := newRecorder(t)
	c, _ := startConsumer(t, "kafka.tenant_consumer.test", rec.args(cluster.ListenAddrs()), nil)

	cl := newProducer(t, cluster.ListenAddrs())
	var bad []testRecord
	for _, r := range allSignals(t, "tenant-a") {
		r.body = []byte("\xff\x00garbage")
		bad = append(bad, r)
	}
	produce(t, cl, bad...)
	produce(t, cl, allSignals(t, "tenant-a")...)

	// Pyroscope bodies are passed through undecoded, so the garbage profile
	// reaches the (fake) writer too: 1 + 4 deliveries.
	require.Eventually(t, func() bool { return len(rec.deliveries()) == 5 }, 20*time.Second, 10*time.Millisecond)
	for _, format := range []string{kafkaclient.FormatPromRWv1, kafkaclient.FormatLokiPush, kafkaclient.FormatOTLP} {
		require.Equal(t, 1.0, testutil.ToFloat64(c.metrics.dropped.WithLabelValues("decode_error", format)), format)
	}
}

func TestConsumer_DownstreamErrorRetriesThenDrops(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	var calls atomic.Int32
	rec := newRecorder(t)
	rec.otelFn = func(string) error {
		if calls.Add(1) <= 3 {
			return errors.New("downstream unavailable")
		}
		return nil
	}
	args := rec.args(cluster.ListenAddrs())
	args.MaxRetries = 2
	c, _ := startConsumer(t, "kafka.tenant_consumer.test", args, nil)

	cl := newProducer(t, cluster.ListenAddrs())
	trace := allSignals(t, "tenant-a")[3]
	produce(t, cl, trace, trace)

	require.Eventually(t, func() bool { return len(rec.deliveries()) == 1 }, 20*time.Second, 10*time.Millisecond)
	require.Equal(t, int32(4), calls.Load(), "3 failed attempts for the first record, 1 for the second")
	require.Equal(t, 1.0, testutil.ToFloat64(c.metrics.dropped.WithLabelValues("downstream_error", kafkaclient.FormatOTLP)))
}

// TestConsumer_StalledPartitionDoesNotBlockOthers checks that a partition
// whose downstream is stuck holds up only itself: other signals of the same
// tenant, and the same signal of other tenants, keep flowing, including
// records fetched after the stall began.
func TestConsumer_StalledPartitionDoesNotBlockOthers(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	release := make(chan struct{})
	rec := newRecorder(t)
	rec.otelFn = func(tenant string) error {
		if tenant == "tenant-a" {
			<-release
		}
		return nil
	}
	startConsumer(t, "kafka.tenant_consumer.test", rec.args(cluster.ListenAddrs()), nil)

	cl := newProducer(t, cluster.ListenAddrs())
	produceRound := func() {
		produce(t, cl, allSignals(t, "tenant-a")...)
		produce(t, cl, allSignals(t, "tenant-b")...)
	}
	const rounds = 3

	// Everything except tenant-a's traces arrives while they are stuck. Later
	// rounds are produced only once the stall has begun, so they must be
	// fetched while it lasts.
	for i := 1; i <= rounds; i++ {
		produceRound()
		require.Eventually(t, func() bool { return len(rec.deliveries()) == i*7 }, 20*time.Second, 10*time.Millisecond, "round %d", i)
	}
	for _, d := range rec.deliveries() {
		require.False(t, d.Tenant == "tenant-a" && d.Format == kafkaclient.FormatOTLP, "tenant-a traces must still be stuck")
	}

	// Once released, the stuck partition catches up in order.
	close(release)
	require.Eventually(t, func() bool { return len(rec.deliveries()) == rounds*8 }, 20*time.Second, 10*time.Millisecond)
}

func TestConsumer_CommitsOnlyAfterProcessing(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	release := make(chan struct{})
	rec := newRecorder(t)
	rec.otelFn = func(string) error { <-release; return nil }
	args := rec.args(cluster.ListenAddrs())
	startConsumer(t, "kafka.tenant_consumer.test", args, nil)

	cl := newProducer(t, cluster.ListenAddrs())
	produce(t, cl, allSignals(t, "tenant-a")[3])

	adm := kadm.NewClient(cl)
	committed := func() int64 {
		offsets, err := adm.FetchOffsets(t.Context(), args.GroupID)
		if err != nil {
			return -1
		}
		o, ok := offsets.Lookup(topicFor(kafkaclient.SignalTraces), 0)
		if !ok {
			return -1
		}
		return o.At
	}

	// The record is being processed but not finished: nothing is committed.
	time.Sleep(2 * time.Second)
	require.Equal(t, int64(-1), committed())

	close(release)
	require.Eventually(t, func() bool { return committed() == 1 }, 20*time.Second, 10*time.Millisecond)
}

func TestConsumer_InvalidRegistryUpdateKeepsPrevious(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	rec := newRecorder(t)
	args := rec.args(cluster.ListenAddrs())
	c, _ := startConsumer(t, "kafka.tenant_consumer.test", args, nil)

	args.Registry.Value = "topics: {logs: t}\npartitions: 4\ntenants: {a: 1}\nretired: [1]"
	require.ErrorContains(t, c.Update(args), "retired partition")

	tenant, ok := c.registry.Load().TenantFor(0)
	require.True(t, ok)
	require.Equal(t, "tenant-a", tenant)
}

func TestConsumer_MissingTopicIsUnhealthy(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics[:3]...))
	defer cluster.Close()

	rec := newRecorder(t)
	c, _ := startConsumer(t, "kafka.tenant_consumer.test", rec.args(cluster.ListenAddrs()), nil)
	require.Eventually(t, func() bool {
		return c.CurrentHealth().Health == component.HealthTypeUnhealthy
	}, 20*time.Second, 10*time.Millisecond)
}
