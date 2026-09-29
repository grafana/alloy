package producer

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
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
  tenant-b: 2
`

func testArgs(brokers []string) Arguments {
	var args Arguments
	args.SetToDefault()
	args.Server.HTTP.ListenAddress = "127.0.0.1"
	args.Server.HTTP.ListenPort = 0
	args.Server.GRPC.ListenAddress = "127.0.0.1"
	args.Server.GRPC.ListenPort = 0
	args.Registry = alloytypes.OptionalSecret{Value: testRegistry}
	args.Client = kafkaclient.Arguments{Brokers: brokers}
	args.MaxBodySize = 1024
	args.ProduceTimeout = 2 * time.Second
	args.Linger = 0
	return args
}

func testOptions(t *testing.T) component.Options {
	return component.Options{
		ID:         "kafka.tenant_producer.test",
		Logger:     util.TestAlloyLogger(t).Slog(),
		Registerer: prometheus.NewRegistry(),
	}
}

// startProducer starts the component and returns an HTTP handler serving its
// routes, bypassing the component's own listener.
func startProducer(t *testing.T, args Arguments) (*Component, http.Handler) {
	t.Helper()
	c, err := New(testOptions(t), args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	router := mux.NewRouter()
	c.mountRoutes(router)
	return c, router
}

func post(h http.Handler, path, tenant string, body []byte, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if tenant != "" {
		req.Header.Set(HeaderOrgID, tenant)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func consumeAll(t *testing.T, brokers []string, n int) []*kgo.Record {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(testTopics...))
	require.NoError(t, err)
	defer cl.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	var out []*kgo.Record
	for len(out) < n {
		fetches := cl.PollFetches(ctx)
		require.NoError(t, ctx.Err())
		out = append(out, fetches.Records()...)
	}
	return out
}

func TestProducer_RoutesToTenantPartition(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	c, h := startProducer(t, testArgs(cluster.ListenAddrs()))
	require.Eventually(t, c.topicOK.Load, 10*time.Second, 10*time.Millisecond)
	require.Equal(t, component.HealthTypeHealthy, c.CurrentHealth().Health)

	resp := post(h, "/api/v1/push?x=1", "tenant-a", []byte("metrics-a"),
		"Content-Type", "application/x-protobuf", "Content-Encoding", "snappy")
	require.Equal(t, http.StatusNoContent, resp.Code)

	resp = post(h, "/ingest?name=app&from=1", "tenant-b", []byte("profile-b"),
		"Content-Type", "application/octet-stream")
	require.Equal(t, http.StatusOK, resp.Code)

	resp = post(h, "/v1/traces", "tenant-b", []byte(`{}`), "Content-Type", "application/json")
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "{}", resp.Body.String())

	records := consumeAll(t, cluster.ListenAddrs(), 3)
	byValue := map[string]*kgo.Record{}
	for _, r := range records {
		byValue[string(r.Value)] = r
	}

	a := byValue["metrics-a"]
	require.NotNil(t, a)
	require.Equal(t, "alloy-metrics", a.Topic)
	require.Equal(t, int32(0), a.Partition)
	require.Equal(t, "tenant-a", string(a.Key))
	require.Equal(t, "tenant-a", kafkaclient.Header(a, kafkaclient.HeaderTenantID))
	require.Equal(t, kafkaclient.SignalMetrics, kafkaclient.Header(a, kafkaclient.HeaderSignal))
	require.Equal(t, kafkaclient.FormatPromRWv1, kafkaclient.Header(a, kafkaclient.HeaderFormat))
	require.Equal(t, "application/x-protobuf", kafkaclient.Header(a, kafkaclient.HeaderContentType))
	require.Equal(t, "snappy", kafkaclient.Header(a, kafkaclient.HeaderContentEncoding))
	require.Equal(t, "/api/v1/push?x=1", kafkaclient.Header(a, kafkaclient.HeaderURL))
	require.Equal(t, kafkaclient.SchemaVersion, kafkaclient.Header(a, kafkaclient.HeaderSchemaVersion))

	b := byValue["profile-b"]
	require.NotNil(t, b)
	require.Equal(t, "alloy-profiles", b.Topic)
	require.Equal(t, int32(2), b.Partition)
	require.Equal(t, kafkaclient.FormatPyroscopeIngest, kafkaclient.Header(b, kafkaclient.HeaderFormat))
	require.Equal(t, "/ingest?name=app&from=1", kafkaclient.Header(b, kafkaclient.HeaderURL))

	tr := byValue["{}"]
	require.NotNil(t, tr)
	require.Equal(t, "alloy-traces", tr.Topic)
	require.Equal(t, int32(2), tr.Partition)
	require.Equal(t, kafkaclient.SignalTraces, kafkaclient.Header(tr, kafkaclient.HeaderSignal))
	require.Equal(t, kafkaclient.FormatOTLP, kafkaclient.Header(tr, kafkaclient.HeaderFormat))
}

func TestProducer_Rejections(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	c, h := startProducer(t, testArgs(cluster.ListenAddrs()))
	require.Eventually(t, c.topicOK.Load, 10*time.Second, 10*time.Millisecond)

	require.Equal(t, http.StatusUnauthorized, post(h, "/loki/api/v1/push", "", []byte("x")).Code)
	require.Equal(t, http.StatusBadRequest, post(h, "/loki/api/v1/push", "tenant-a|tenant-b", []byte("x")).Code)
	require.Equal(t, http.StatusForbidden, post(h, "/loki/api/v1/push", "tenant-x", []byte("x")).Code)
	require.Equal(t, http.StatusRequestEntityTooLarge, post(h, "/loki/api/v1/push", "tenant-a", bytes.Repeat([]byte("x"), 2048)).Code)
}

func TestProducer_SignalWithoutTopic(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, "alloy-logs"))
	defer cluster.Close()

	args := testArgs(cluster.ListenAddrs())
	args.Registry = alloytypes.OptionalSecret{Value: "topics: {logs: alloy-logs}\npartitions: 4\ntenants: {tenant-a: 0}"}
	c, h := startProducer(t, args)
	require.Eventually(t, c.topicOK.Load, 10*time.Second, 10*time.Millisecond)

	require.Equal(t, http.StatusNotFound, post(h, "/ingest", "tenant-a", []byte("x")).Code)
	require.Equal(t, http.StatusNoContent, post(h, "/loki/api/v1/push", "tenant-a", []byte("x")).Code)
}

func TestProducer_BrokerDown(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))

	c, h := startProducer(t, testArgs(cluster.ListenAddrs()))
	require.Eventually(t, c.topicOK.Load, 10*time.Second, 10*time.Millisecond)

	cluster.Close()
	resp := post(h, "/v1/logs", "tenant-a", []byte("x"))
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestProducer_MissingTopic(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics[:3]...))
	defer cluster.Close()

	c, h := startProducer(t, testArgs(cluster.ListenAddrs()))
	require.Eventually(t, func() bool {
		health := c.CurrentHealth()
		return health.Health == component.HealthTypeUnhealthy && strings.Contains(health.Message, "does not exist")
	}, 10*time.Second, 10*time.Millisecond)

	require.Equal(t, http.StatusServiceUnavailable, post(h, "/v1/logs", "tenant-a", []byte("x")).Code)
}

func TestProducer_TooFewPartitions(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(2, testTopics...))
	defer cluster.Close()

	c, _ := startProducer(t, testArgs(cluster.ListenAddrs()))
	require.Eventually(t, func() bool {
		return strings.Contains(c.CurrentHealth().Message, "has 2 partitions but the tenant registry declares 4")
	}, 10*time.Second, 10*time.Millisecond)
}

func TestArguments_Unmarshal(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		registry      = "topics: {logs: t}\npartitions: 1\n"
		max_body_size = "512KiB"
		compression   = "zstd"
		client {
			brokers = ["localhost:9092"]
			sasl {
				username = "u"
				password = "p"
			}
		}
	`), &args))
	require.Equal(t, 512*units.KiB, args.MaxBodySize)
	require.Equal(t, "PLAIN", args.Client.SASL.Mechanism)

	require.ErrorContains(t, syntax.Unmarshal([]byte(`
		registry = ""
		client { brokers = [] }
	`), &args), "at least one broker")
	require.ErrorContains(t, syntax.Unmarshal([]byte(`
		registry = ""
		client {
			brokers = ["x"]
			sasl {
				mechanism = "GSSAPI"
				username  = "u"
				password  = "p"
			}
		}
	`), &args), "unsupported SASL mechanism")
	require.ErrorContains(t, syntax.Unmarshal([]byte(`
		registry    = ""
		compression = "brotli"
		client { brokers = ["x"] }
	`), &args), "unsupported compression")
}

func TestProducer_InvalidRegistryUpdateKeepsPrevious(t *testing.T) {
	cluster := kfake.MustCluster(kfake.SeedTopics(4, testTopics...))
	defer cluster.Close()

	args := testArgs(cluster.ListenAddrs())
	c, _ := startProducer(t, args)

	args.Registry = alloytypes.OptionalSecret{Value: "topics: {logs: t}\npartitions: 4\ntenants: {a: 1, b: 1}"}
	require.ErrorContains(t, c.Update(args), "assigned to both")

	p, ok := c.registry.Load().PartitionFor("tenant-b")
	require.True(t, ok)
	require.Equal(t, int32(2), p)
}
