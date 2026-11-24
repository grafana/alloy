package source

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
)

const testID = "infinity.source.test"

func testOptions(t *testing.T, cl cluster.Cluster) component.Options {
	return component.Options{
		ID:         testID,
		Logger:     util.TestLogger(t),
		Registerer: prometheus.NewRegistry(),
		GetServiceData: func(name string) (any, error) {
			switch name {
			case cluster.ServiceName:
				return cl, nil
			case labelstore.ServiceName:
				return labelstore.New(nil, prometheus.NewRegistry()), nil
			}
			return nil, fmt.Errorf("no service %q", name)
		},
	}
}

// receivers are the outputs of a test component. A nil field adds no
// output of that type.
type receivers struct {
	app  storage.Appender
	otel otelcol.Consumer
	logs loki.LogsReceiver
}

// testArgs parses cfg and adds the outputs in o. New needs a metrics output,
// so a zero o adds a collecting appender that the test does not read.
func testArgs(t *testing.T, cfg string, o receivers) Arguments {
	t.Helper()
	args, err := parse(cfg)
	require.NoError(t, err)
	if o.app == nil && o.otel == nil && o.logs == nil {
		o.app = testappender.NewCollectingAppender()
	}
	if o.app != nil {
		args.ForwardTo.Metrics = []storage.AppendableV2{testappender.ConstantAppendable{Inner: o.app}}
	}
	if o.otel != nil {
		args.Output.Metrics = []otelcol.Consumer{o.otel}
	}
	if o.logs != nil {
		args.ForwardTo.Logs = []loki.LogsReceiver{o.logs}
	}
	return args
}

// newTestComponent builds a component from cfg and o. It does not run it.
func newTestComponent(t *testing.T, opts component.Options, cfg string, o receivers) *Component {
	t.Helper()
	c, err := New(opts, testArgs(t, cfg, o))
	require.NoError(t, err)
	return c
}

// startTestComponent builds a component and runs it until the test ends.
func startTestComponent(t *testing.T, opts component.Options, cfg string, o receivers) *Component {
	t.Helper()
	c := newTestComponent(t, opts, cfg, o)
	go func() { _ = c.Run(t.Context()) }()
	return c
}

// runComponent runs c until the returned cancel is called. The returned
// channel closes when Run returns.
func runComponent(parent context.Context, c *Component) (context.CancelFunc, <-chan struct{}) {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Run(ctx)
	}()
	return cancel, done
}

// countParses makes c count its parses, which is one per owned poll. Call it
// before Run.
func countParses(c *Component) *atomic.Int32 {
	parses := atomic.NewInt32(0)
	c.parse = func(s querySpec, b []byte) (*data.Frame, error) {
		parses.Inc()
		return buildFrame(s, b)
	}
	return parses
}

func series(name string, extra ...string) string {
	return labels.FromStrings(append([]string{"__name__", name, "job", testID, "instance", "q"}, extra...)...).String()
}

// switchable serves the current body, or the current status when it is not 200.
type switchable struct {
	body   atomic.Value
	status atomic.Int32
	hits   atomic.Int32
}

func (s *switchable) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s.hits.Add(1)
	if code := int(s.status.Load()); code != 0 && code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	_, _ = w.Write([]byte(s.body.Load().(string)))
}

func queryConfig(url string) string {
	return fmt.Sprintf(`
		interval = "100ms"
		timeout  = "50ms"
		query "q" {
			url = %q
		}`, url)
}

// inlineQueryConfig is a config with one inline table query named name.
func inlineQueryConfig(interval, name string) string {
	return fmt.Sprintf(`
		interval = %q
		timeout  = "50ms"
		query %q {
			source = "inline"
			data   = "[{\"v\":1}]"
		}`, interval, name)
}

func TestMetricsEndToEnd(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"name":"a","v":1},{"name":"b","v":2}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	startTestComponent(t, testOptions(t, cluster.Mock()), queryConfig(srv.URL), receivers{app: app})

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		a := app.LatestSampleFor(series("v", "name", "a"))
		up := app.LatestSampleFor(series("up"))
		if !assert.NotNil(c, a) || !assert.NotNil(c, up) {
			return
		}
		assert.Equal(c, 1.0, a.Value)
		assert.Equal(c, 1.0, up.Value)
	}, 2*time.Second, 20*time.Millisecond)

	// Series b vanishes, so it gets a stale marker.
	h.body.Store(`[{"name":"a","v":1}]`)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		b := app.LatestSampleFor(series("v", "name", "b"))
		if assert.NotNil(c, b) {
			assert.True(c, value.IsStaleNaN(b.Value))
		}
	}, 2*time.Second, 20*time.Millisecond)
}

func TestFailedPollMarksStaleAndDown(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"name":"a","v":1}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cluster.Mock()), queryConfig(srv.URL), receivers{app: app})
	require.Eventually(t, func() bool { return app.LatestSampleFor(series("v", "name", "a")) != nil }, 2*time.Second, 20*time.Millisecond)

	h.status.Store(http.StatusServiceUnavailable)
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		a := app.LatestSampleFor(series("v", "name", "a"))
		up := app.LatestSampleFor(series("up"))
		if !assert.NotNil(ct, a) || !assert.NotNil(ct, up) {
			return
		}
		assert.True(ct, value.IsStaleNaN(a.Value))
		assert.Equal(ct, 0.0, up.Value)
		health := c.CurrentHealth()
		assert.Equal(ct, component.HealthTypeUnhealthy, health.Health)
		assert.Contains(ct, health.Message, `query "q" failed: status 503`)
	}, 2*time.Second, 20*time.Millisecond)
}

func TestHealthMultipleFailuresMessage(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	cfg := fmt.Sprintf(`
		interval = "100ms"
		timeout  = "50ms"
		query "a" {
			url = %q
		}
		query "b" {
			url = %q
		}`, srv.URL, srv.URL)

	c := startTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{})

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		h := c.CurrentHealth()
		assert.Equal(ct, component.HealthTypeUnhealthy, h.Health)
		assert.Contains(ct, h.Message, `query "a" failed: status 404 (and 1 other query)`)
	}, 2*time.Second, 20*time.Millisecond)
}

func TestHealthRedactsURL(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + "/api?api_key=supersecret"
	srv.Close()

	c := startTestComponent(t, testOptions(t, cluster.Mock()), queryConfig(url), receivers{})

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		h := c.CurrentHealth()
		assert.Equal(ct, component.HealthTypeUnhealthy, h.Health)
		assert.NotContains(ct, h.Message, "supersecret")
	}, 2*time.Second, 20*time.Millisecond)
}

func TestLogsInline(t *testing.T) {
	recv := loki.NewLogsReceiver()
	cfg := `
		interval = "100ms"
		timeout  = "50ms"
		query "q" {
			type   = "csv"
			source = "inline"
			format = "logs"
			data   = "level,body\nerror,disk full\n"
		}`
	startTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{logs: recv})

	select {
	case e := <-recv.Chan():
		require.Equal(t, "disk full", e.Line)
		require.EqualValues(t, testID, e.Labels["job"])
		require.EqualValues(t, "q", e.Labels["instance"])
		require.EqualValues(t, "error", e.Labels["level"])
	case <-time.After(2 * time.Second):
		t.Fatal("no log entry")
	}
}

func TestJobInstanceOverride(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"job":"from-api","v":1}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	startTestComponent(t, testOptions(t, cluster.Mock()), queryConfig(srv.URL), receivers{app: app})
	require.Eventually(t, func() bool { return app.LatestSampleFor(series("v")) != nil }, 2*time.Second, 20*time.Millisecond)
}

func TestUpColumnReserved(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"up": false}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	startTestComponent(t, testOptions(t, cluster.Mock()), queryConfig(srv.URL), receivers{app: app})

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		up := app.LatestSampleFor(series("up"))
		if !assert.NotNil(c, up) {
			return
		}
		// The response's own "up" column must not override the synthetic
		// up sample: only the poll's success (1) shows up here.
		assert.Equal(c, 1.0, up.Value)
	}, 2*time.Second, 20*time.Millisecond)
}

func TestUpdateRemovesQuery(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"v":1}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cluster.Mock()), queryConfig(srv.URL), receivers{app: app})
	require.Eventually(t, func() bool { return app.LatestSampleFor(series("v")) != nil }, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, c.Update(testArgs(t, fmt.Sprintf(`
		interval = "100ms"
		timeout  = "50ms"
		query "other" { url = %q }`, srv.URL), receivers{app: app})))

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		v := app.LatestSampleFor(series("v"))
		up := app.LatestSampleFor(series("up"))
		if !assert.NotNil(ct, v) || !assert.NotNil(ct, up) {
			return
		}
		assert.True(ct, value.IsStaleNaN(v.Value))
		assert.True(ct, value.IsStaleNaN(up.Value))
	}, 2*time.Second, 20*time.Millisecond)
}

// otherOwner is a cluster where another node owns every key.
type otherOwner struct{ cluster.Cluster }

func (otherOwner) Lookup(shard.Key, int, shard.Op) ([]peer.Peer, error) {
	return []peer.Peer{{Name: "other", Self: false}}, nil
}

func TestClusteringSkipsForeignQueries(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"v":1}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	cfg := queryConfig(srv.URL) + "\nclustering { enabled = true }"
	startTestComponent(t, testOptions(t, otherOwner{cluster.Mock()}), cfg, receivers{})

	time.Sleep(400 * time.Millisecond)
	require.Zero(t, h.hits.Load(), "this node does not own the query")
}

func TestNewRejectsMissingOutputs(t *testing.T) {
	args, err := parse(queryConfig("http://example.com"))
	require.NoError(t, err)
	_, err = New(testOptions(t, cluster.Mock()), args)
	require.ErrorContains(t, err, "needs forward_to.metrics or output.metrics")
}

// TestNewRetriesAfterFailedBuild checks that New works on a registry where
// a failed New already registered the self-metrics. Alloy keeps the
// registry of a component across a failed build.
func TestNewRetriesAfterFailedBuild(t *testing.T) {
	opts := testOptions(t, cluster.Mock())
	reg := prometheus.NewRegistry()
	opts.Registerer = reg

	args, err := parse(inlineQueryConfig("100ms", "q"))
	require.NoError(t, err)
	_, err = New(opts, args)
	require.ErrorContains(t, err, "needs forward_to.metrics or output.metrics")

	c := startTestComponent(t, opts, inlineQueryConfig("100ms", "q"), receivers{})

	// The registry must show the counts of the second component.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		n, err := promtestutil.GatherAndCount(reg, "infinity_source_samples_sent_total")
		assert.NoError(ct, err)
		assert.Equal(ct, 1, n)
		assert.GreaterOrEqual(ct, promtestutil.ToFloat64(c.metrics.samplesSent.WithLabelValues("q")), 2.0)
		// The fanout registers its own metrics. They must count the
		// samples of the second component too.
		forwarded, err := counterSum(reg, "prometheus_forwarded_samples_total")
		assert.NoError(ct, err)
		assert.GreaterOrEqual(ct, forwarded, 2.0)
	}, 2*time.Second, 10*time.Millisecond)
}

// counterSum returns the sum of all series of the counter name in reg.
func counterSum(reg prometheus.Gatherer, name string) (float64, error) {
	mfs, err := reg.Gather()
	if err != nil {
		return 0, err
	}
	var sum float64
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			sum += m.GetCounter().GetValue()
		}
	}
	return sum, nil
}

// TestCancelledPollSendsNoMarkers guards against a poll that is cancelled
// mid-fetch (shutdown, or a reload that stops its loop) faking an outage by
// sending up=0 and stale markers.
func TestCancelledPollSendsNoMarkers(t *testing.T) {
	entered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	// timeout must be <= interval, so both hold the poll open long enough
	// for the test to cancel it mid-flight.
	cfg := fmt.Sprintf(`
		interval = "2s"
		timeout  = "2s"
		query "q" {
			url = %q
		}`, srv.URL)
	c := newTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{app: app})
	cancel, _ := runComponent(context.Background(), c)
	defer cancel()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler was never entered")
	}
	cancel()

	require.Never(t, func() bool {
		return app.LatestSampleFor(series("up")) != nil
	}, 300*time.Millisecond, 20*time.Millisecond, "a cancelled poll must not send up=0 or a stale marker")
}

// TestUpdateDoesNotWaitForSlowPoll guards against Update waiting for a slow
// poll. A wait would also block CurrentHealth and the other queries' polls.
func TestUpdateDoesNotWaitForSlowPoll(t *testing.T) {
	var once sync.Once
	block := make(chan struct{})
	closeBlock := func() { once.Do(func() { close(block) }) }
	defer closeBlock()

	entered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		// Also end on client disconnect, so srv.Close cannot hang if the
		// test fails before it closes block.
		select {
		case <-block:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`[{"v":1}]`))
	}))
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	// timeout must be <= interval. Both stay short so the random initial
	// offset (up to interval) does not make the test slow.
	cfg := fmt.Sprintf(`
		interval = "2s"
		timeout  = "2s"
		query "q" {
			url = %q
		}`, srv.URL)
	c := startTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{app: app})

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler was never entered")
	}

	args := testArgs(t, fmt.Sprintf(`
		interval = "2s"
		timeout  = "2s"
		query "q" {
			url = %q
		}`, srv.URL+"/other"), receivers{app: app})
	updateDone := make(chan struct{})
	var updateErr error
	go func() {
		defer close(updateDone)
		updateErr = c.Update(args)
	}()

	// The poll is still blocked, so Update must not wait for it.
	select {
	case <-updateDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Update waited for a slow poll")
	}
	require.NoError(t, updateErr)

	start := time.Now()
	_ = c.CurrentHealth()
	require.Less(t, time.Since(start), 100*time.Millisecond, "CurrentHealth must not wait for a slow poll")
}

// TestParserPanicFailsOnlyThatQuery checks that a response that makes the
// parser panic marks its own query unhealthy and leaves other queries alone.
func TestParserPanicFailsOnlyThatQuery(t *testing.T) {
	cfg := `
		interval = "100ms"
		timeout  = "50ms"
		query "bad" {
			source = "inline"
			data   = "[{\"a\":1},{\"a\":\"x\"}]"
		}
		query "q" {
			source = "inline"
			data   = "[{\"v\":1}]"
		}`
	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{app: app})

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		h := c.CurrentHealth()
		assert.Equal(ct, component.HealthTypeUnhealthy, h.Health)
		assert.Contains(ct, h.Message, `query "bad" failed: the parser failed on the response`)
		assert.NotContains(ct, h.Message, "other query")
		v := app.LatestSampleFor(series("v"))
		if assert.NotNil(ct, v) {
			assert.Equal(ct, 1.0, v.Value)
		}
	}, 2*time.Second, 20*time.Millisecond)
}

// TestParseBoundedByTimeout uses a jq expression that runs far longer than
// timeout (0.8s, or 15s with -race, when measured) and then ends,
// so it does not leave a goroutine that uses CPU for the rest of the tests.
// The poll must fail with reason timeout, and Run must still stop at once.
func TestParseBoundedByTimeout(t *testing.T) {
	c := newTestComponent(t, testOptions(t, cluster.Mock()), `
		interval = "1s"
		timeout  = "200ms"
		query "q" {
			source        = "inline"
			data          = "[1]"
			parser        = "jq-backend"
			root_selector = "reduce range(0; 10000000) as $i (0; .+1)"
		}`, receivers{})
	cancel, runDone := runComponent(t.Context(), c)
	defer cancel()

	// The first poll starts within one interval and must end one timeout later.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, 1.0, promtestutil.ToFloat64(c.metrics.pollFailures.WithLabelValues("q", reasonTimeout)))
		h := c.CurrentHealth()
		assert.Contains(ct, h.Message, "the request or parsing took longer than timeout")
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case <-runDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

// TestFormatChangeToLogsMarksStale checks that a query that changes from
// table to logs sends stale markers for its old series, including up.
func TestFormatChangeToLogsMarksStale(t *testing.T) {
	table := `
		interval = "100ms"
		timeout  = "50ms"
		query "q" {
			type   = "csv"
			source = "inline"
			data   = "name,v\na,1\n"
			column {
				selector = "name"
			}
			column {
				selector = "v"
				type     = "number"
			}
		}`
	app := testappender.NewCollectingAppender()
	recv := loki.NewLogsReceiver()
	go func() {
		for {
			select {
			case <-recv.Chan():
			case <-t.Context().Done():
				return
			}
		}
	}()
	c := startTestComponent(t, testOptions(t, cluster.Mock()), table, receivers{app: app, logs: recv})
	require.Eventually(t, func() bool {
		up := app.LatestSampleFor(series("up"))
		return up != nil && up.Value == 1
	}, 2*time.Second, 20*time.Millisecond)

	logs := `
		interval = "100ms"
		timeout  = "50ms"
		query "q" {
			type   = "csv"
			source = "inline"
			format = "logs"
			data   = "level,body\nerror,disk full\n"
		}`
	require.NoError(t, c.Update(testArgs(t, logs, receivers{app: app, logs: recv})))

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		v := app.LatestSampleFor(series("v", "name", "a"))
		up := app.LatestSampleFor(series("up"))
		if !assert.NotNil(ct, v) || !assert.NotNil(ct, up) {
			return
		}
		assert.True(ct, value.IsStaleNaN(v.Value))
		assert.True(ct, value.IsStaleNaN(up.Value))
	}, 2*time.Second, 20*time.Millisecond)
}

// TestUpdateClosesOldClient checks that Update closes the idle connections
// of the HTTP client it replaces.
func TestUpdateClosesOldClient(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"v":1}]`)
	srv := httptest.NewUnstartedServer(h)
	var closed atomic.Int32
	srv.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateClosed {
			closed.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	cfg := fmt.Sprintf(`
		interval = "100ms"
		timeout  = "100ms"
		query "q" {
			url = %q
		}`, srv.URL)
	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{app: app})
	require.Eventually(t, func() bool { return h.hits.Load() > 0 }, 2*time.Second, 10*time.Millisecond)

	// A long interval keeps the new client from polling, so only the old
	// client's connection can close.
	require.NoError(t, c.Update(testArgs(t, fmt.Sprintf(`
		interval = "1h"
		timeout  = "100ms"
		query "q" {
			url = %q
		}`, srv.URL), receivers{app: app})))

	require.Eventually(t, func() bool { return closed.Load() > 0 }, 2*time.Second, 10*time.Millisecond)
}

// TestClientAcceptReplacesDefault checks that an Accept header in
// client.http_headers is the only Accept value the server gets.
func TestClientAcceptReplacesDefault(t *testing.T) {
	accept := make(chan []string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case accept <- r.Header.Values("Accept"):
		default:
		}
		_, _ = w.Write([]byte(`[{"v":1}]`))
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`
		interval = "100ms"
		timeout  = "50ms"
		client {
			http_headers = {
				"Accept" = ["application/vnd.api+json"],
			}
		}
		query "q" {
			url = %q
		}`, srv.URL)
	startTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{})

	select {
	case got := <-accept:
		require.Equal(t, []string{"application/vnd.api+json"}, got)
	case <-time.After(2 * time.Second):
		t.Fatal("no request")
	}
}

// TestOAuthTokenHangIsBounded uses a token server that never answers.
// prometheus/common gets tokens without a deadline, so the poll must still
// end at timeout.
func TestOAuthTokenHangIsBounded(t *testing.T) {
	release := make(chan struct{})
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	defer tokenSrv.Close()
	defer close(release)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"v":1}]`))
	}))
	defer api.Close()

	c := newTestComponent(t, testOptions(t, cluster.Mock()), fmt.Sprintf(`
		interval = "1s"
		timeout  = "200ms"
		client {
			oauth2 {
				client_id     = "id"
				client_secret = "secret"
				token_url     = %q
			}
		}
		query "q" {
			url = %q
		}`, tokenSrv.URL, api.URL), receivers{})
	cancel, runDone := runComponent(t.Context(), c)
	defer cancel()

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, 1.0, promtestutil.ToFloat64(c.metrics.pollFailures.WithLabelValues("q", reasonTimeout)))
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case <-runDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

// TestOneAbandonedWorkerPerQuery checks that a parse that never ends blocks
// later polls of that query instead of starting more workers. The parse
// blocks on a channel instead of running an endless jq expression, so the
// test leaves no goroutine that uses CPU.
func TestOneAbandonedWorkerPerQuery(t *testing.T) {
	c := newTestComponent(t, testOptions(t, cluster.Mock()), `
		interval = "100ms"
		timeout  = "50ms"
		query "q" {
			source = "inline"
			data   = "[1]"
		}`, receivers{})

	release := make(chan struct{})
	defer close(release)
	var parses atomic.Int32
	c.parse = func(querySpec, []byte) (*data.Frame, error) {
		parses.Inc()
		<-release
		return nil, errors.New("released")
	}
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.GreaterOrEqual(ct, promtestutil.ToFloat64(c.metrics.pollFailures.WithLabelValues("q", reasonTimeout)), 4.0)
		assert.Contains(ct, c.CurrentHealth().Message, "the previous poll is still running")
	}, 3*time.Second, 10*time.Millisecond)

	c.mut.RLock()
	qs := c.queries["q"]
	c.mut.RUnlock()
	require.Equal(t, int32(1), qs.workers.Load())
	require.Equal(t, int32(1), parses.Load())
}

// TestAbandonedWorkerSurvivesRemoveAndReadd checks that a query keeps its
// worker count when a reload removes it and a later reload adds it back.
// Otherwise each such pair of reloads could leave one more worker behind.
func TestAbandonedWorkerSurvivesRemoveAndReadd(t *testing.T) {
	out := receivers{app: testappender.NewCollectingAppender(), otel: &testConsumer{}}
	withQ := testArgs(t, inlineQueryConfig("100ms", "q"), out)
	withoutQ := testArgs(t, inlineQueryConfig("100ms", "other"), out)
	c, err := New(testOptions(t, cluster.Mock()), withQ)
	require.NoError(t, err)

	release := make(chan struct{})
	defer close(release)
	var parses atomic.Int32
	c.parse = func(s querySpec, b []byte) (*data.Frame, error) {
		if s.name != "q" {
			return buildFrame(s, b)
		}
		parses.Inc()
		<-release
		return nil, errors.New("released")
	}
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Contains(ct, c.CurrentHealth().Message, "the request or parsing took longer than timeout")
	}, 3*time.Second, 10*time.Millisecond)

	require.NoError(t, c.Update(withoutQ))
	require.Nil(t, queryStateOf(c, "q"))
	require.NoError(t, c.Update(withQ))

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Contains(ct, c.CurrentHealth().Message, "the previous poll is still running")
	}, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, int32(1), parses.Load(), "a re-added query must not start a second worker")
}

// TestOutputsReadAtEmitTime checks that a poll sends to the outputs that are
// current when it emits, not the ones from when it started.
func TestOutputsReadAtEmitTime(t *testing.T) {
	var once sync.Once
	block := make(chan struct{})
	closeBlock := func() { once.Do(func() { close(block) }) }
	defer closeBlock()
	entered := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-block:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`[{"v":1}]`))
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`
		interval = "2s"
		timeout  = "2s"
		query "q" {
			url = %q
		}`, srv.URL)
	first, second := &testConsumer{}, &testConsumer{}
	c := newTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{otel: first})
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler was never entered")
	}
	require.NoError(t, c.Update(testArgs(t, cfg, receivers{otel: second})))
	closeBlock()

	require.Eventually(t, func() bool {
		m, _ := second.snapshot()
		return len(m) > 0
	}, 2*time.Second, 10*time.Millisecond)
	m, _ := first.snapshot()
	require.Empty(t, m, "the poll must not send to the old output")
}

// TestCancelledEmitSendsNothing checks that a poll whose context ends after
// a successful fetch sends nothing and keeps the tracker as it was.
func TestCancelledEmitSendsNothing(t *testing.T) {
	for _, format := range []string{formatTable, formatLogs} {
		t.Run(format, func(t *testing.T) {
			app := testappender.NewCollectingAppender()
			recv := loki.NewLogsReceiver()
			c := newTestComponent(t, testOptions(t, cluster.Mock()), fmt.Sprintf(`
				query "q" {
					source = "inline"
					format = %q
					data   = "[{\"v\":1}]"
				}`, format), receivers{app: app, logs: recv})

			c.mut.RLock()
			qs := c.queries["q"]
			c.mut.RUnlock()
			g := c.gen.Load()
			s := g.specs["q"]
			frame, err := buildFrame(s, []byte(s.data))
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			qs.mut.Lock()
			err = c.emit(ctx, qs, s, time.Now(), frame, nil)
			tracked := qs.tracker.len()
			qs.mut.Unlock()

			require.Error(t, err)
			require.Zero(t, tracked)
			require.Nil(t, app.LatestSampleFor(series("up")))
			select {
			case <-recv.Chan():
				t.Fatal("a cancelled poll sent a log entry")
			default:
			}
		})
	}
}

// TestHealthRedactsRedirectURL checks that the health message and the warn
// logs show no part of a bad redirect Location.
func TestHealthRedactsRedirectURL(t *testing.T) {
	for name, location := range secretRedirects {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(redirectTo(location))
			defer srv.Close()

			logs := newLogRecorder()
			opts := testOptions(t, cluster.Mock())
			opts.Logger = slog.New(logs)
			c := startTestComponent(t, opts, queryConfig(srv.URL), receivers{})

			require.EventuallyWithT(t, func(ct *assert.CollectT) {
				assert.Equal(ct, component.HealthTypeUnhealthy, c.CurrentHealth().Health)
				assert.NotEmpty(ct, logs.at(slog.LevelWarn))
			}, 2*time.Second, 20*time.Millisecond)

			// Read the health first. Each poll logs before it sets the
			// health, so the warn lines then include this message.
			msg := c.CurrentHealth().Message
			warns := logs.at(slog.LevelWarn)
			require.Contains(t, warnMessages(warns), strings.TrimPrefix(msg, `query "q" failed: `))
			for _, part := range forbiddenParts(location) {
				require.NotContains(t, msg, part)
				for _, line := range warns {
					require.NotContains(t, line, part)
				}
			}
			// Debug logs keep the net/http text, which can show the Location.
			debugs := logs.at(slog.LevelDebug)
			require.NotEmpty(t, debugs)
			require.Contains(t, debugs[0], `detail=`)
		})
	}
}

// warnMessages returns the err field of each "poll failed" line.
func warnMessages(lines []string) []string {
	var msgs []string
	for _, line := range lines {
		rest, ok := strings.CutPrefix(line, `poll failed `)
		if !ok {
			continue
		}
		if _, v, ok := strings.Cut(rest, ` err="`); ok {
			msgs = append(msgs, strings.TrimSuffix(v, `"`))
		}
	}
	return msgs
}

// switchOwner is a cluster where this node owns every key until other is set.
type switchOwner struct {
	cluster.Cluster
	other   *atomic.Bool
	lookups *atomic.Int32
	gate    *lookupGate
}

func newSwitchOwner(other bool) switchOwner {
	return switchOwner{Cluster: cluster.Mock(), other: atomic.NewBool(other), lookups: atomic.NewInt32(0), gate: &lookupGate{}}
}

// Lookup reads the owner before it waits at the gate, so a held Lookup
// returns the owner from before any later move.
func (s switchOwner) Lookup(shard.Key, int, shard.Op) ([]peer.Peer, error) {
	s.lookups.Inc()
	other := s.other.Load()
	s.gate.wait()
	if other {
		return []peer.Peer{{Name: "other", Self: false}}, nil
	}
	return []peer.Peer{{Name: "self", Self: true}}, nil
}

// lookupGate holds one Lookup until the test releases it.
type lookupGate struct {
	mu      sync.Mutex
	entered chan struct{}
	release chan struct{}
}

// arm makes the next Lookup wait. entered closes when that Lookup waits.
// release lets it return, and is safe to call more than once.
func (g *lookupGate) arm() (entered <-chan struct{}, release func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entered = make(chan struct{})
	g.release = make(chan struct{})
	r := g.release
	var once sync.Once
	return g.entered, func() { once.Do(func() { close(r) }) }
}

func (g *lookupGate) wait() {
	g.mu.Lock()
	entered, release := g.entered, g.release
	g.entered, g.release = nil, nil
	g.mu.Unlock()
	if entered == nil {
		return
	}
	close(entered)
	<-release
}

// move gives every key to another node, or back to this node, and tells c,
// the same way the cluster service does.
func (s switchOwner) move(toOther bool, c *Component) {
	s.other.Store(toOther)
	c.NotifyClusterChange()
}

// TestRemovedQueryMarkersOnlyFromOwner checks that a node that no longer
// owns a query sends no stale markers when the query is removed. The new
// owner still sends those series.
func TestRemovedQueryMarkersOnlyFromOwner(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"v":1}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	cl := newSwitchOwner(false)
	// A long interval leaves time to remove the query before the next poll.
	cfg := fmt.Sprintf(`
		interval = "2s"
		timeout  = "1s"
		clustering {
			enabled = true
		}
		query "q" {
			url = %q
		}`, srv.URL)
	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cl), cfg, receivers{app: app})
	require.Eventually(t, func() bool { return app.LatestSampleFor(series("v")) != nil }, 3*time.Second, 10*time.Millisecond)

	cl.other.Store(true)
	require.NoError(t, c.Update(testArgs(t, fmt.Sprintf(`
		interval = "2s"
		timeout  = "1s"
		clustering {
			enabled = true
		}
		query "other" {
			url = %q
		}`, srv.URL), receivers{app: app})))

	v := app.LatestSampleFor(series("v"))
	require.NotNil(t, v)
	require.False(t, value.IsStaleNaN(v.Value), "a node that lost ownership must not mark the series stale")
}

// TestFailedPollCountsUp checks that the up=0 sample of a failed poll is
// counted in infinity_source_samples_sent_total.
func TestFailedPollCountsUp(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	c := startTestComponent(t, testOptions(t, cluster.Mock()), queryConfig(srv.URL), receivers{})

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.GreaterOrEqual(ct, promtestutil.ToFloat64(c.metrics.samplesSent.WithLabelValues("q")), 1.0)
	}, 2*time.Second, 20*time.Millisecond)
}

// clusteredInline is a clustered config with one inline query. interval is
// long, so only a kick can make the query poll during a test.
const clusteredInline = `
	interval = "10s"
	timeout  = "1s"
	clustering {
		enabled = true
	}
	query "q" {
		source = "inline"
		data   = "[{\"v\":1}]"
	}`

func queryStateOf(c *Component, name string) *queryState {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.queries[name]
}

// TestClusterChangeGainPollsSoon checks that a node that gains a query polls
// it within interval/10, not at its next tick.
func TestClusterChangeGainPollsSoon(t *testing.T) {
	cl := newSwitchOwner(true)
	c := newTestComponent(t, testOptions(t, cl), clusteredInline, receivers{})
	parses := countParses(c)
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()

	cl.move(false, c)
	require.Eventually(t, func() bool { return parses.Load() > 0 }, 2*time.Second, 10*time.Millisecond)
}

// TestClusterChangeLossStopsPolling checks that a node that loses a query
// stops polling it, clears its series without stale markers, and clears
// its health.
func TestClusterChangeLossStopsPolling(t *testing.T) {
	h := &switchable{}
	h.status.Store(http.StatusServiceUnavailable)
	srv := httptest.NewServer(h)
	defer srv.Close()

	cl := newSwitchOwner(false)
	cfg := queryConfig(srv.URL) + `
		clustering {
			enabled = true
		}`
	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cl), cfg, receivers{app: app})
	qs := queryStateOf(c, "q")
	require.Eventually(t, func() bool {
		return c.CurrentHealth().Health == component.HealthTypeUnhealthy
	}, 2*time.Second, 10*time.Millisecond)

	cl.move(true, c)
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		qs.mut.Lock()
		tracked := qs.tracker.len()
		qs.mut.Unlock()
		assert.Zero(ct, tracked)
		assert.Equal(ct, component.HealthTypeHealthy, c.CurrentHealth().Health)
	}, 2*time.Second, 10*time.Millisecond)

	hits := h.hits.Load()
	require.Never(t, func() bool { return h.hits.Load() > hits }, 400*time.Millisecond, 20*time.Millisecond, "a node that lost the query must not poll it")
	up := app.LatestSampleFor(series("up"))
	require.NotNil(t, up)
	require.False(t, value.IsStaleNaN(up.Value), "a node that lost the query must not send stale markers")
}

// TestNotifyClusterChangeDoesNotWaitForPoll checks that NotifyClusterChange
// returns at once, and that ownership still changes, while a poll is blocked.
func TestNotifyClusterChangeDoesNotWaitForPoll(t *testing.T) {
	var once sync.Once
	block := make(chan struct{})
	closeBlock := func() { once.Do(func() { close(block) }) }
	defer closeBlock()
	entered := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	cl := newSwitchOwner(false)
	cfg := fmt.Sprintf(`
		interval = "2s"
		timeout  = "2s"
		clustering {
			enabled = true
		}
		query "q" {
			url = %q
		}`, srv.URL)
	c := startTestComponent(t, testOptions(t, cl), cfg, receivers{})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler was never entered")
	}

	qs := queryStateOf(c, "q")
	require.True(t, qs.assigned.Load())

	start := time.Now()
	for range 100 {
		c.NotifyClusterChange()
	}
	cl.move(true, c)
	require.Less(t, time.Since(start), 50*time.Millisecond, "NotifyClusterChange must not wait for a poll")

	require.Eventually(t, func() bool { return !qs.assigned.Load() }, time.Second, 5*time.Millisecond, "a blocked poll must not delay the ownership change")
}

// TestNotifyClusterChangeDisabled checks that NotifyClusterChange does
// nothing when clustering is disabled, and does not panic before Run.
func TestNotifyClusterChangeDisabled(t *testing.T) {
	cl := newSwitchOwner(false)
	c := newTestComponent(t, testOptions(t, cl), `
		query "q" {
			source = "inline"
			data   = "[1]"
		}`, receivers{})

	require.NotPanics(t, c.NotifyClusterChange)
	require.Zero(t, len(c.clusterChanged), "a disabled component must not queue a cluster change")
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()
	c.NotifyClusterChange()
	time.Sleep(50 * time.Millisecond)
	require.Zero(t, cl.lookups.Load(), "a disabled component must not look up owners")

	c = newTestComponent(t, testOptions(t, cluster.Mock()), clusteredInline, receivers{})
	require.NotPanics(t, func() {
		c.NotifyClusterChange()
		c.NotifyClusterChange()
	})
}

// TestNotifyClusterChangeCoalesces checks that notifications that come
// during a refresh start no goroutines and give one more refresh only.
func TestNotifyClusterChangeCoalesces(t *testing.T) {
	cl := newSwitchOwner(false)
	c := startTestComponent(t, testOptions(t, cl), clusteredInline, receivers{})
	time.Sleep(100 * time.Millisecond)

	before := runtime.NumGoroutine()
	lookups := cl.lookups.Load()
	// Hold the refresh of the first notification, so the others queue.
	entered, release := cl.gate.arm()
	defer release()
	c.NotifyClusterChange()
	<-entered
	for range 10000 {
		c.NotifyClusterChange()
	}
	release()

	require.Eventually(t, func() bool { return len(c.clusterChanged) == 0 }, time.Second, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	require.LessOrEqual(t, goroutinesAfter(before, time.Second), before, "notifications must start no goroutines")
	// One query, so each refresh makes one Lookup.
	require.LessOrEqual(t, cl.lookups.Load()-lookups, int32(3), "notifications must coalesce")
}

// TestRunStopsPendingJitter checks that Run returns at once, runs no poll,
// and leaves no goroutine when a kicked poll waits for its jitter.
func TestRunStopsPendingJitter(t *testing.T) {
	before := runtime.NumGoroutine()
	cl := newSwitchOwner(true)
	cfg := `
		interval = "1h"
		timeout  = "1s"
		clustering {
			enabled = true
		}
		query "q" {
			source = "inline"
			data   = "[{\"v\":1}]"
		}`
	c := newTestComponent(t, testOptions(t, cl), cfg, receivers{})
	parses := countParses(c)
	cancel, runDone := runComponent(t.Context(), c)
	defer cancel()

	// The jitter is up to 6 minutes, so it is still pending at shutdown.
	cl.move(false, c)
	qs := queryStateOf(c, "q")
	require.Eventually(t, func() bool {
		return qs.assigned.Load() && len(qs.kick) == 0 && len(c.clusterChanged) == 0
	}, time.Second, 5*time.Millisecond)

	cancel()
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
	require.Zero(t, parses.Load())
	require.LessOrEqual(t, goroutinesAfter(before, 2*time.Second), before, "Run must leave no goroutine")
}

// goroutinesAfter waits until at most want goroutines run, or until timeout,
// and returns the count. require.Eventually runs its condition in a new
// goroutine, so it cannot count them.
func goroutinesAfter(want int, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for {
		n := runtime.NumGoroutine()
		if n <= want || time.Now().After(deadline) {
			return n
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRefreshesAreSerialized checks that an Update refresh that uses an old
// ring cannot overwrite a newer watcher refresh.
func TestRefreshesAreSerialized(t *testing.T) {
	cl := newSwitchOwner(true)
	c := startTestComponent(t, testOptions(t, cl), clusteredInline, receivers{})
	time.Sleep(100 * time.Millisecond)

	args := testArgs(t, clusteredInline+`
		query "b" {
			source = "inline"
			data   = "[1]"
		}`, receivers{})

	// Update looks up b with the old ring and waits.
	entered, release := cl.gate.arm()
	defer release()
	updateErr := make(chan error, 1)
	go func() { updateErr <- c.Update(args) }()
	<-entered

	// The ring moves b to this node while Update waits.
	cl.move(false, c)
	time.Sleep(100 * time.Millisecond)
	release()
	require.NoError(t, <-updateErr)

	b := queryStateOf(c, "b")
	require.Eventually(t, func() bool { return b.assigned.Load() }, time.Second, 5*time.Millisecond, "the newest ring must win")
	require.Never(t, func() bool { return !b.assigned.Load() }, 200*time.Millisecond, 10*time.Millisecond)
}

// TestOwnershipLostDuringFetchDropsResult checks that a poll whose node
// loses the query during a slow fetch sends nothing. The new owner writes
// the same series, so up=0, stale markers or data from this node would
// conflict with it.
func TestOwnershipLostDuringFetchDropsResult(t *testing.T) {
	tests := map[string]struct {
		qtype  string
		format string
		status int
		body   string
	}{
		"table failure": {qtype: "json", format: formatTable, status: http.StatusInternalServerError},
		"table success": {qtype: "json", format: formatTable, status: http.StatusOK, body: `[{"v":1}]`},
		"logs success":  {qtype: "csv", format: formatLogs, status: http.StatusOK, body: "level,body\nerror,disk full\n"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var once sync.Once
			block := make(chan struct{})
			closeBlock := func() { once.Do(func() { close(block) }) }
			defer closeBlock()
			entered := make(chan struct{}, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case entered <- struct{}{}:
				default:
				}
				select {
				case <-block:
				case <-r.Context().Done():
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			cl := newSwitchOwner(false)
			cfg := fmt.Sprintf(`
				interval = "2s"
				timeout  = "2s"
				clustering {
					enabled = true
				}
				query "q" {
					type   = %q
					url    = %q
					format = %q
				}`, tc.qtype, srv.URL, tc.format)
			app := testappender.NewCollectingAppender()
			recv := loki.NewLogsReceiver()
			c := startTestComponent(t, testOptions(t, cl), cfg, receivers{app: app, logs: recv})
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("handler was never entered")
			}

			qs := queryStateOf(c, "q")
			cl.move(true, c)
			require.Eventually(t, func() bool { return !qs.assigned.Load() }, time.Second, 5*time.Millisecond)
			closeBlock()

			select {
			case e := <-recv.Chan():
				t.Fatalf("a node that lost the query sent a log entry: %q", e.Line)
			case <-time.After(300 * time.Millisecond):
			}
			require.Empty(t, app.CollectedSamples(), "a node that lost the query must send no samples")
			require.Eventually(t, func() bool {
				qs.mut.Lock()
				defer qs.mut.Unlock()
				return qs.tracker.len() == 0
			}, time.Second, 5*time.Millisecond, "the poll must give up the query after the fetch")
			require.Equal(t, component.HealthTypeHealthy, c.CurrentHealth().Health)
		})
	}
}

// TestClusteringEnabledDuringFetchDropsResult checks that a reload that
// turns clustering on during a fetch, and keeps the interval, makes the
// poll drop its result when another node owns the query. The loop keeps
// running, so the poll must read the clustering setting after the fetch.
func TestClusteringEnabledDuringFetchDropsResult(t *testing.T) {
	var (
		hits      atomic.Int32
		once      sync.Once
		block     = make(chan struct{})
		entered   = make(chan struct{}, 1)
		closeOnce = func() { once.Do(func() { close(block) }) }
	)
	defer closeOnce()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The first poll fails, so the health has an error to clear.
		if hits.Inc() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-block:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`[{"v":1}]`))
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`
		interval = "1s"
		timeout  = "1s"
		query "q" {
			url = %q
		}`, srv.URL)
	cl := newSwitchOwner(true)
	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cl), cfg, receivers{app: app})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the second poll never reached the server")
	}
	require.Equal(t, component.HealthTypeUnhealthy, c.CurrentHealth().Health)

	require.NoError(t, c.Update(testArgs(t, cfg+"\nclustering { enabled = true }", receivers{app: app})))
	qs := queryStateOf(c, "q")
	upBefore := app.LatestSampleFor(series("up"))
	require.NotNil(t, upBefore)
	closeOnce()

	require.Eventually(t, func() bool {
		qs.mut.Lock()
		defer qs.mut.Unlock()
		return qs.tracker.len() == 0
	}, 2*time.Second, 5*time.Millisecond, "the poll must give up the query after the fetch")
	// No data, no new up sample and no stale markers.
	require.Nil(t, app.LatestSampleFor(series("v")), "a node that does not own the query must send no data")
	upAfter := app.LatestSampleFor(series("up"))
	require.Equal(t, upBefore.Timestamp, upAfter.Timestamp, "a node that does not own the query must send no up sample or stale marker")
	require.Equal(t, component.HealthTypeHealthy, c.CurrentHealth().Health)
}

// checkingAppender calls check before each sample that it passes on. Use a
// pointer, because the fanout compares its children and a func is not
// comparable.
type checkingAppender struct {
	storage.Appender
	check func()
}

func (a *checkingAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.check()
	return a.Appender.Append(ref, l, t, v)
}

// TestClusteringEnabledNoGapBeforeOwnership checks that a reload that turns
// clustering on sets the ownership of the kept queries before the new
// setting takes effect. Another node owns every query, so this node must
// send no sample while clustering is on. The fetch ends while the reload
// waits in an ownership lookup.
func TestClusteringEnabledNoGapBeforeOwnership(t *testing.T) {
	var (
		once      sync.Once
		block     = make(chan struct{})
		entered   = make(chan struct{}, 1)
		closeOnce = func() { once.Do(func() { close(block) }) }
	)
	defer closeOnce()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-block:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`[{"v":1}]`))
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`
		interval = "1s"
		timeout  = "1s"
		query "q" {
			url = %q
		}`, srv.URL)
	cl := newSwitchOwner(true)
	app := testappender.NewCollectingAppender()
	var (
		comp      atomic.Pointer[Component]
		nonOwnerN atomic.Int32
	)
	out := receivers{app: &checkingAppender{Appender: app, check: func() {
		if c := comp.Load(); c != nil && c.gen.Load().clustering {
			nonOwnerN.Inc()
		}
	}}}
	c := newTestComponent(t, testOptions(t, cl), cfg, out)
	comp.Store(c)
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the poll never reached the server")
	}

	lookupEntered, releaseLookup := cl.gate.arm()
	defer releaseLookup()
	args := testArgs(t, cfg+"\nclustering { enabled = true }", out)
	updateDone := make(chan error, 1)
	go func() { updateDone <- c.Update(args) }()
	select {
	case <-lookupEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("the reload never looked up an owner")
	}

	// The fetch ends while the reload waits for the lookup.
	closeOnce()
	require.Eventually(t, func() bool {
		qs := queryStateOf(c, "q")
		qs.mut.Lock()
		defer qs.mut.Unlock()
		return app.LatestSampleFor(series("v")) != nil || qs.tracker.len() == 0
	}, 2*time.Second, 5*time.Millisecond, "the poll must end")
	releaseLookup()
	select {
	case err := <-updateDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Update did not return")
	}
	require.Zero(t, nonOwnerN.Load(), "a node that does not own the query sent samples while clustering was on")
}

// TestUpdateSendsRemovalMarkersToOldOutputs checks that a reload that
// removes a query and replaces the metric outputs sends the query's stale
// markers to the outputs from before the reload. Only those outputs have
// the query's series.
func TestUpdateSendsRemovalMarkersToOldOutputs(t *testing.T) {
	oldApp, newApp := testappender.NewCollectingAppender(), testappender.NewCollectingAppender()
	oldOTel, newOTel := &testConsumer{}, &testConsumer{}
	c := startTestComponent(t, testOptions(t, cluster.Mock()), inlineQueryConfig("100ms", "q"), receivers{app: oldApp, otel: oldOTel})
	require.Eventually(t, func() bool { return oldApp.LatestSampleFor(series("up")) != nil }, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, c.Update(testArgs(t, inlineQueryConfig("100ms", "other"), receivers{app: newApp, otel: newOTel})))

	for _, name := range []string{"v", "up"} {
		s := oldApp.LatestSampleFor(series(name))
		require.NotNil(t, s, "series %s", name)
		require.True(t, value.IsStaleNaN(s.Value), "the old output must get a stale marker for %s", name)
		require.Nil(t, newApp.LatestSampleFor(series(name)), "the new output must not get series %s", name)
	}
	_, stale := oldOTel.pointsFor("q")
	require.Equal(t, 2, stale, "the old OTel output must get a stale marker for v and up")
	points, _ := newOTel.pointsFor("q")
	require.Zero(t, points, "the new OTel output must get nothing for the removed query")
}

// TestUpdateRemovalMarkersDoNotBlockReload checks that a reload that removes
// a query returns when the old OTel consumer blocks. The send of the stale
// markers has a timeout.
func TestUpdateRemovalMarkersDoNotBlockReload(t *testing.T) {
	app := testappender.NewCollectingAppender()
	c := startTestComponent(t, testOptions(t, cluster.Mock()), inlineQueryConfig("1h", "q"), receivers{app: app, otel: &blockingConsumer{}})
	// A 1h interval waits up to 1h for the first poll, so poll now.
	c.mut.RLock()
	qs := c.queries["q"]
	c.mut.RUnlock()
	c.poll(t.Context(), "q", qs)
	require.NotNil(t, app.LatestSampleFor(series("up")))

	args := testArgs(t, inlineQueryConfig("1h", "other"), receivers{})
	done := make(chan error, 1)
	go func() { done <- c.Update(args) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Update did not return while the old OTel consumer blocked")
	}
	require.True(t, value.IsStaleNaN(app.LatestSampleFor(series("up")).Value))
}

// TestUpdateOutputsOnlySendsNoMarkers checks that a reload that changes
// only the outputs sends no stale markers. The query keeps its series and
// sends them to the new outputs.
func TestUpdateOutputsOnlySendsNoMarkers(t *testing.T) {
	cfg := inlineQueryConfig("100ms", "q")
	oldApp, newApp := testappender.NewCollectingAppender(), testappender.NewCollectingAppender()
	oldOTel, newOTel := &testConsumer{}, &testConsumer{}
	c := startTestComponent(t, testOptions(t, cluster.Mock()), cfg, receivers{app: oldApp, otel: oldOTel})
	require.Eventually(t, func() bool { return oldApp.LatestSampleFor(series("up")) != nil }, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, c.Update(testArgs(t, cfg, receivers{app: newApp, otel: newOTel})))
	require.Eventually(t, func() bool { return newApp.LatestSampleFor(series("up")) != nil }, 2*time.Second, 10*time.Millisecond)

	for _, name := range []string{"v", "up"} {
		s := oldApp.LatestSampleFor(series(name))
		require.NotNil(t, s, "series %s", name)
		require.False(t, value.IsStaleNaN(s.Value), "a reload of the outputs must not mark %s stale", name)
	}
	_, stale := oldOTel.pointsFor("q")
	require.Zero(t, stale, "a reload of the outputs must send no OTel stale markers")
	_, stale = newOTel.pointsFor("q")
	require.Zero(t, stale, "a reload of the outputs must send no OTel stale markers")
}

// TestUpdateIntervalAndRemovalMarksOldOutputs checks a reload that changes
// the interval, removes a query and replaces the outputs at once. The
// removed query's stale markers go to the outputs from before the reload.
func TestUpdateIntervalAndRemovalMarksOldOutputs(t *testing.T) {
	oldApp, newApp := testappender.NewCollectingAppender(), testappender.NewCollectingAppender()
	oldOTel, newOTel := &testConsumer{}, &testConsumer{}
	c := startTestComponent(t, testOptions(t, cluster.Mock()), inlineQueryConfig("100ms", "q"), receivers{app: oldApp, otel: oldOTel})
	require.Eventually(t, func() bool { return oldApp.LatestSampleFor(series("up")) != nil }, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, c.Update(testArgs(t, inlineQueryConfig("200ms", "other"), receivers{app: newApp, otel: newOTel})))

	for _, name := range []string{"v", "up"} {
		s := oldApp.LatestSampleFor(series(name))
		require.NotNil(t, s, "series %s", name)
		require.True(t, value.IsStaleNaN(s.Value), "the old output must get a stale marker for %s", name)
		require.Nil(t, newApp.LatestSampleFor(series(name)), "the new output must not get series %s", name)
	}
	_, stale := oldOTel.pointsFor("q")
	require.Equal(t, 2, stale, "the old OTel output must get a stale marker for v and up")
	points, _ := newOTel.pointsFor("q")
	require.Zero(t, points, "the new OTel output must get nothing for the removed query")
	require.Eventually(t, func() bool {
		points, _ := newOTel.pointsFor("other")
		return points > 0
	}, 2*time.Second, 10*time.Millisecond, "the new query must send to the new OTel output")
}

// TestSendFailureWarnsHideReceiverText checks the warn lines of a failed send
// for a poll and for stale markers. A receiver error can hold remote text,
// so these lines must hold only the fixed message.
func TestSendFailureWarnsHideReceiverText(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"v":1}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	logs := newLogRecorder()
	opts := testOptions(t, cluster.Mock())
	opts.Logger = slog.New(logs)
	startTestComponent(t, opts, queryConfig(srv.URL), receivers{otel: &failingConsumer{}})

	const pollLine = `poll failed query="q" reason="emit" err="could not send the result to the outputs"`
	require.Eventually(t, func() bool { return slices.Contains(logs.at(slog.LevelWarn), pollLine) }, 2*time.Second, 10*time.Millisecond)

	// A failed fetch sends up = 0 with stale markers, which also fails.
	h.status.Store(http.StatusServiceUnavailable)
	const staleLine = `failed to send stale markers query="q" err="could not send the result to the outputs"`
	require.Eventually(t, func() bool { return slices.Contains(logs.at(slog.LevelWarn), staleLine) }, 2*time.Second, 10*time.Millisecond)

	for _, line := range logs.at(slog.LevelWarn) {
		require.NotContains(t, line, "supersecret")
		require.NotContains(t, line, "r.example")
	}
}
