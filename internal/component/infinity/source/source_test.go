package source

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
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

// TestParseBoundedByTimeout uses a jq expression that never ends. The poll
// must fail with reason timeout, and Run must still stop at once.
func TestParseBoundedByTimeout(t *testing.T) {
	args, err := parse(`
		interval = "1s"
		timeout  = "200ms"
		query "q" {
			source        = "inline"
			data          = "[1]"
			parser        = "jq-backend"
			root_selector = "until(false; .)"
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
		assert.Contains(ct, h.Message, "parsing the response took longer than timeout")
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
