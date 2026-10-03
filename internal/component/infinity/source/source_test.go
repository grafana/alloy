package source

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
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

// startComponent parses cfg, adds app as the metrics receiver and recv as the
// logs receiver, and runs the component until ctx ends.
func startComponent(ctx context.Context, opts component.Options, cfg string, app storage.Appendable, recv loki.LogsReceiver) (*Component, error) {
	args, err := parse(cfg)
	if err != nil {
		return nil, err
	}
	if app != nil {
		args.ForwardTo.Metrics = []storage.Appendable{app}
	}
	if recv != nil {
		args.ForwardTo.Logs = []loki.LogsReceiver{recv}
	}
	c, err := New(opts, args)
	if err != nil {
		return nil, err
	}
	go func() { _ = c.Run(ctx) }()
	return c, nil
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

func TestMetricsEndToEnd(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"name":"a","v":1},{"name":"b","v":2}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	_, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(srv.URL), testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

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
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(srv.URL), testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)
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

	app := testappender.NewCollectingAppender()
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), cfg, testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

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

	app := testappender.NewCollectingAppender()
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(url), testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

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
	_, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), cfg, nil, recv)
	require.NoError(t, err)

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
	_, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(srv.URL), testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return app.LatestSampleFor(series("v")) != nil }, 2*time.Second, 20*time.Millisecond)
}

func TestUpColumnReserved(t *testing.T) {
	h := &switchable{}
	h.body.Store(`[{"up": false}]`)
	srv := httptest.NewServer(h)
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	_, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(srv.URL), testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

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
	appendable := testappender.ConstantAppendable{Inner: app}
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(srv.URL), appendable, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return app.LatestSampleFor(series("v")) != nil }, 2*time.Second, 20*time.Millisecond)

	args, err := parse(fmt.Sprintf(`
		interval = "100ms"
		timeout  = "50ms"
		query "other" { url = %q }`, srv.URL))
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{appendable}
	require.NoError(t, c.Update(args))

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
	app := testappender.NewCollectingAppender()
	_, err := startComponent(t.Context(), testOptions(t, otherOwner{cluster.Mock()}), cfg, testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

	time.Sleep(400 * time.Millisecond)
	require.Zero(t, h.hits.Load(), "this node does not own the query")
}

func TestNewRejectsMissingOutputs(t *testing.T) {
	args, err := parse(queryConfig("http://example.com"))
	require.NoError(t, err)
	_, err = New(testOptions(t, cluster.Mock()), args)
	require.ErrorContains(t, err, "needs forward_to.metrics or output.metrics")
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// timeout must be <= interval, so both hold the poll open long enough
	// for the test to cancel it mid-flight.
	cfg := fmt.Sprintf(`
		interval = "2s"
		timeout  = "2s"
		query "q" {
			url = %q
		}`, srv.URL)
	_, err := startComponent(ctx, testOptions(t, cluster.Mock()), cfg, testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

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
	appendable := testappender.ConstantAppendable{Inner: app}
	// timeout must be <= interval. Both stay short so the random initial
	// offset (up to interval) does not make the test slow.
	cfg := fmt.Sprintf(`
		interval = "2s"
		timeout  = "2s"
		query "q" {
			url = %q
		}`, srv.URL)
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), cfg, appendable, nil)
	require.NoError(t, err)

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler was never entered")
	}

	updateDone := make(chan struct{})
	var updateErr error
	go func() {
		defer close(updateDone)
		args, perr := parse(fmt.Sprintf(`
			interval = "2s"
			timeout  = "2s"
			query "q" {
				url = %q
			}`, srv.URL+"/other"))
		if perr != nil {
			updateErr = perr
			return
		}
		args.ForwardTo.Metrics = []storage.Appendable{appendable}
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
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), cfg, testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		h := c.CurrentHealth()
		assert.Equal(ct, component.HealthTypeUnhealthy, h.Health)
		assert.Contains(ct, h.Message, `query "bad" failed: parser panic`)
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
	args, err := parse(`
		interval = "1s"
		timeout  = "200ms"
		query "q" {
			source        = "inline"
			data          = "[1]"
			parser        = "jq-backend"
			root_selector = "reduce range(0; 10000000) as $i (0; .+1)"
		}`)
	require.NoError(t, err)
	app := testappender.NewCollectingAppender()
	args.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{Inner: app}}
	c, err := New(testOptions(t, cluster.Mock()), args)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		_ = c.Run(ctx)
	}()

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
	appendable := testappender.ConstantAppendable{Inner: app}
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
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), table, appendable, recv)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		up := app.LatestSampleFor(series("up"))
		return up != nil && up.Value == 1
	}, 2*time.Second, 20*time.Millisecond)

	args, err := parse(`
		interval = "100ms"
		timeout  = "50ms"
		query "q" {
			type   = "csv"
			source = "inline"
			format = "logs"
			data   = "level,body\nerror,disk full\n"
		}`)
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{appendable}
	args.ForwardTo.Logs = []loki.LogsReceiver{recv}
	require.NoError(t, c.Update(args))

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
	appendable := testappender.ConstantAppendable{Inner: app}
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), cfg, appendable, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return h.hits.Load() > 0 }, 2*time.Second, 10*time.Millisecond)

	// A long interval keeps the new client from polling, so only the old
	// client's connection can close.
	args, err := parse(fmt.Sprintf(`
		interval = "1h"
		timeout  = "100ms"
		query "q" {
			url = %q
		}`, srv.URL))
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{appendable}
	require.NoError(t, c.Update(args))

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
	app := testappender.NewCollectingAppender()
	_, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), cfg, testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

	select {
	case got := <-accept:
		require.Equal(t, []string{"application/vnd.api+json"}, got)
	case <-time.After(2 * time.Second):
		t.Fatal("no request")
	}
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

	args, err := parse(fmt.Sprintf(`
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
		}`, tokenSrv.URL, api.URL))
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}}
	c, err := New(testOptions(t, cluster.Mock()), args)
	require.NoError(t, err)
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
	args, err := parse(`
		interval = "100ms"
		timeout  = "50ms"
		query "q" {
			source = "inline"
			data   = "[1]"
		}`)
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}}
	c, err := New(testOptions(t, cluster.Mock()), args)
	require.NoError(t, err)

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
	args, err := parse(cfg)
	require.NoError(t, err)
	args.Output.Metrics = []otelcol.Consumer{first}
	c, err := New(testOptions(t, cluster.Mock()), args)
	require.NoError(t, err)
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler was never entered")
	}
	args, err = parse(cfg)
	require.NoError(t, err)
	args.Output.Metrics = []otelcol.Consumer{second}
	require.NoError(t, c.Update(args))
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
			args, err := parse(fmt.Sprintf(`
				query "q" {
					source = "inline"
					format = %q
					data   = "[{\"v\":1}]"
				}`, format))
			require.NoError(t, err)
			app := testappender.NewCollectingAppender()
			recv := loki.NewLogsReceiver()
			args.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{Inner: app}}
			args.ForwardTo.Logs = []loki.LogsReceiver{recv}
			c, err := New(testOptions(t, cluster.Mock()), args)
			require.NoError(t, err)

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

func TestHealthRedactsRedirectURL(t *testing.T) {
	for name, location := range secretRedirects {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(redirectTo(location))
			defer srv.Close()

			app := testappender.NewCollectingAppender()
			c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(srv.URL), testappender.ConstantAppendable{Inner: app}, nil)
			require.NoError(t, err)

			require.EventuallyWithT(t, func(ct *assert.CollectT) {
				h := c.CurrentHealth()
				assert.Equal(ct, component.HealthTypeUnhealthy, h.Health)
				assert.NotContains(ct, h.Message, "supersecret")
			}, 2*time.Second, 20*time.Millisecond)
		})
	}
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
	appendable := testappender.ConstantAppendable{Inner: app}
	c, err := startComponent(t.Context(), testOptions(t, cl), cfg, appendable, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return app.LatestSampleFor(series("v")) != nil }, 3*time.Second, 10*time.Millisecond)

	cl.other.Store(true)
	args, err := parse(fmt.Sprintf(`
		interval = "2s"
		timeout  = "1s"
		clustering {
			enabled = true
		}
		query "other" {
			url = %q
		}`, srv.URL))
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{appendable}
	require.NoError(t, c.Update(args))

	v := app.LatestSampleFor(series("v"))
	require.NotNil(t, v)
	require.False(t, value.IsStaleNaN(v.Value), "a node that lost ownership must not mark the series stale")
}

// TestFailedPollCountsUp checks that the up=0 sample of a failed poll is
// counted in infinity_source_samples_sent_total.
func TestFailedPollCountsUp(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	app := testappender.NewCollectingAppender()
	c, err := startComponent(t.Context(), testOptions(t, cluster.Mock()), queryConfig(srv.URL), testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)

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

// newCounting builds a component from cfg that sends metrics to app. The
// returned counter counts the parses, which is one per owned poll.
func newCounting(opts component.Options, cfg string, app storage.Appendable) (*Component, *atomic.Int32, error) {
	args, err := parse(cfg)
	if err != nil {
		return nil, nil, err
	}
	args.ForwardTo.Metrics = []storage.Appendable{app}
	c, err := New(opts, args)
	if err != nil {
		return nil, nil, err
	}
	parses := atomic.NewInt32(0)
	c.parse = func(s querySpec, b []byte) (*data.Frame, error) {
		parses.Inc()
		return buildFrame(s, b)
	}
	return c, parses, nil
}

func queryStateOf(c *Component, name string) *queryState {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.queries[name]
}

// TestClusterChangeGainPollsSoon checks that a node that gains a query polls
// it within interval/10, not at its next tick.
func TestClusterChangeGainPollsSoon(t *testing.T) {
	cl := newSwitchOwner(true)
	c, parses, err := newCounting(testOptions(t, cl), clusteredInline, testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})
	require.NoError(t, err)
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
	c, err := startComponent(t.Context(), testOptions(t, cl), cfg, testappender.ConstantAppendable{Inner: app}, nil)
	require.NoError(t, err)
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
	c, err := startComponent(t.Context(), testOptions(t, cl), cfg, testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}, nil)
	require.NoError(t, err)
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
	args, err := parse(`
		query "q" {
			source = "inline"
			data   = "[1]"
		}`)
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}}
	cl := newSwitchOwner(false)
	c, err := New(testOptions(t, cl), args)
	require.NoError(t, err)

	require.NotPanics(t, c.NotifyClusterChange)
	require.Zero(t, len(c.clusterChanged), "a disabled component must not queue a cluster change")
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()
	c.NotifyClusterChange()
	time.Sleep(50 * time.Millisecond)
	require.Zero(t, cl.lookups.Load(), "a disabled component must not look up owners")

	c, _, err = newCounting(testOptions(t, cluster.Mock()), clusteredInline, testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})
	require.NoError(t, err)
	require.NotPanics(t, func() {
		c.NotifyClusterChange()
		c.NotifyClusterChange()
	})
}

// TestNotifyClusterChangeCoalesces checks that notifications that come
// during a refresh start no goroutines and give one more refresh only.
func TestNotifyClusterChangeCoalesces(t *testing.T) {
	cl := newSwitchOwner(false)
	c, _, err := newCounting(testOptions(t, cl), clusteredInline, testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})
	require.NoError(t, err)
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()
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
	c, parses, err := newCounting(testOptions(t, cl), cfg, testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})
	require.NoError(t, err)
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
	c, _, err := newCounting(testOptions(t, cl), clusteredInline, testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})
	require.NoError(t, err)
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()
	time.Sleep(100 * time.Millisecond)

	args, err := parse(clusteredInline + `
		query "b" {
			source = "inline"
			data   = "[1]"
		}`)
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}}

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

// TestUpdateKicksGainedRunningLoop checks that Update kicks a new query that
// it finds owned when Run already started its loop.
func TestUpdateKicksGainedRunningLoop(t *testing.T) {
	cl := newSwitchOwner(false)
	c, _, err := newCounting(testOptions(t, cl), clusteredInline, testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()})
	require.NoError(t, err)
	var parsesB atomic.Int32
	c.parse = func(s querySpec, body []byte) (*data.Frame, error) {
		if s.name == "b" {
			parsesB.Inc()
		}
		return buildFrame(s, body)
	}

	args, err := parse(clusteredInline + `
		query "b" {
			source = "inline"
			data   = "[1]"
		}`)
	require.NoError(t, err)
	args.ForwardTo.Metrics = []storage.Appendable{testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}}

	// Update adds b and waits in its Lookup. Run then starts the loop of b.
	entered, release := cl.gate.arm()
	defer release()
	updateErr := make(chan error, 1)
	go func() { updateErr <- c.Update(args) }()
	<-entered
	cancel, _ := runComponent(t.Context(), c)
	defer cancel()
	require.Eventually(t, func() bool {
		c.mut.RLock()
		defer c.mut.RUnlock()
		b := c.queries["b"]
		return b != nil && b.loop != nil
	}, time.Second, 5*time.Millisecond)
	release()
	require.NoError(t, <-updateErr)

	// interval is 10s, so only a kick polls b this soon.
	require.Eventually(t, func() bool { return parsesB.Load() > 0 }, 2*time.Second, 10*time.Millisecond)
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
			c, err := startComponent(t.Context(), testOptions(t, cl), cfg, testappender.ConstantAppendable{Inner: app}, recv)
			require.NoError(t, err)
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
				return !qs.owned && qs.tracker.len() == 0
			}, time.Second, 5*time.Millisecond, "the poll must give up the query after the fetch")
			require.Equal(t, component.HealthTypeHealthy, c.CurrentHealth().Health)
		})
	}
}
