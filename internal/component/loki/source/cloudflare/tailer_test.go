package cloudflare

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestTailer(t *testing.T) {
	rangeKey := func(start, end time.Time) string {
		return strconv.FormatInt(start.UnixNano(), 10) + "-" + strconv.FormatInt(end.UnixNano(), 10)
	}

	var (
		logger = logging.NewSlogNop()
		end    = time.Unix(0, time.Hour.Nanoseconds())
		start  = time.Unix(0, time.Hour.Nanoseconds()-int64(time.Minute))
		third  = time.Minute / 3
		// Lines served for each pull request of the first window, keyed by start and end.
		responses = map[string]string{
			rangeKey(start, start.Add(third)):              `{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`,
			rangeKey(start.Add(third), start.Add(2*third)): `{"EdgeStartTimestamp":2, "EdgeRequestHost":"bar.com"}`,
			rangeKey(start.Add(2*third), end):              `{"EdgeStartTimestamp":3, "EdgeRequestHost":"buzz.com"}` + "\n" + `{"EdgeRequestHost":"fuzz.com"}`,
		}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		_, _ = io.WriteString(w, responses[q.Get("start")+"-"+q.Get("end")])
	}))
	defer srv.Close()

	var (
		cfg = &tailerConfig{
			APIToken:   "foo",
			ZoneID:     "bar",
			APIURL:     srv.URL,
			Labels:     model.LabelSet{"job": "cloudflare"},
			PullRange:  model.Duration(time.Minute),
			FieldsType: FieldsTypeDefault,
			Workers:    3,
			Backoff:    defaultBackoff,
		}
		handler = loki.NewCollectingHandler()
	)
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	// set our end time to be the last time we have a position
	ps.Put(positions.CursorKey(cfg.ZoneID), cfg.Labels.String(), end.UnixNano())
	require.NoError(t, err)

	ta, err := newTailer(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, cfg)
	require.NoError(t, err)
	require.True(t, ta.ready())

	require.Eventually(t, func() bool {
		return len(handler.Received()) == 4
	}, 5*time.Second, 100*time.Millisecond)

	received := handler.Received()
	sort.Slice(received, func(i, j int) bool {
		return received[i].Timestamp.After(received[j].Timestamp)
	})
	for _, e := range received {
		require.Equal(t, model.LabelValue("cloudflare"), e.Labels["job"])
	}
	require.WithinDuration(t, time.Now(), received[0].Timestamp, time.Minute) // no timestamp default to now.
	require.Equal(t, `{"EdgeRequestHost":"fuzz.com"}`, received[0].Line)

	require.Equal(t, `{"EdgeStartTimestamp":3, "EdgeRequestHost":"buzz.com"}`, received[1].Line)
	require.Equal(t, time.Unix(0, 3), received[1].Timestamp)
	require.Equal(t, `{"EdgeStartTimestamp":2, "EdgeRequestHost":"bar.com"}`, received[2].Line)
	require.Equal(t, time.Unix(0, 2), received[2].Timestamp)
	require.Equal(t, `{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`, received[3].Line)
	require.Equal(t, time.Unix(0, 1), received[3].Timestamp)
	ta.stop()
	ps.Stop()
	// Make sure we save the last position.
	newPos, _ := ps.Get(positions.CursorKey(cfg.ZoneID), cfg.Labels.String())
	require.Greater(t, newPos, end.UnixNano())
}

func TestTailer_RetryErrorLogpullReceived(t *testing.T) {
	var (
		logger  = logging.NewSlogNop()
		end     = time.Unix(0, time.Hour.Nanoseconds())
		start   = time.Unix(0, end.Add(-30*time.Minute).UnixNano())
		handler = loki.NewCollectingHandler()
		calls   atomic.Int32
	)
	// Fail the first request, then succeed.
	client, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if calls.Inc() == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "error")
		}
	}, nil)
	defer srv.Close()
	ta := &tailer{
		logger:  logger,
		handler: handler.Receiver(),
		client:  client,
		config: &tailerConfig{
			Labels: make(model.LabelSet),
			Backoff: backoff.Config{
				MinBackoff: 0,
				MaxBackoff: 0,
				MaxRetries: 5,
			},
		},
		metrics: newMetrics(nil),
	}

	require.NoError(t, ta.pull(t.Context(), start, end))
	require.Equal(t, int32(2), calls.Load())
}

func TestTailer_RetryErrorIterating(t *testing.T) {
	var (
		logger  = logging.NewSlogNop()
		end     = time.Unix(0, time.Hour.Nanoseconds())
		start   = time.Unix(0, end.Add(-30*time.Minute).UnixNano())
		handler = loki.NewCollectingHandler()
		calls   atomic.Int32
	)
	client, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if calls.Inc() == 1 {
			// Declaring a longer body than is written makes the client fail mid-stream.
			w.Header().Set("Content-Length", "1000")
			_, _ = io.WriteString(w, `{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`+"\n")
			return
		}
		_, _ = io.WriteString(w, `{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`+"\n"+
			`{"EdgeStartTimestamp":2, "EdgeRequestHost":"foo.com"}`+"\n"+
			`{"EdgeStartTimestamp":3, "EdgeRequestHost":"foo.com"}`+"\n")
	}, nil)
	defer srv.Close()
	ta := &tailer{
		logger:  logger,
		handler: handler.Receiver(),
		client:  client,
		config: &tailerConfig{
			Labels: make(model.LabelSet),
			Backoff: backoff.Config{
				MinBackoff: 0,
				MaxBackoff: 0,
				MaxRetries: 5,
			},
		},
		metrics: newMetrics(prometheus.NewRegistry()),
	}

	require.NoError(t, ta.pull(t.Context(), start, end))
	require.Eventually(t, func() bool {
		return len(handler.Received()) == 4
	}, 5*time.Second, 100*time.Millisecond)
}

func TestTailer_CloudflareTargetError(t *testing.T) {
	var calls atomic.Int32
	// A client error is not retried by the client, so every retry is the tailer's.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Inc()
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "no logs")
	}))
	defer srv.Close()

	var (
		logger = logging.NewSlogNop()
		cfg    = &tailerConfig{
			APIToken:   "foo",
			ZoneID:     "bar",
			APIURL:     srv.URL,
			Labels:     model.LabelSet{"job": "cloudflare"},
			PullRange:  model.Duration(time.Minute),
			FieldsType: FieldsTypeDefault,
			Workers:    3,
			Backoff:    backoff.Config{MinBackoff: 0, MaxBackoff: 0, MaxRetries: 5},
		}
		end     = time.Unix(0, time.Hour.Nanoseconds())
		handler = loki.NewCollectingHandler()
	)
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})

	// set our end time to be the last time we have a position
	ps.Put(positions.CursorKey(cfg.ZoneID), cfg.Labels.String(), end.UnixNano())
	require.NoError(t, err)

	ta, err := newTailer(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, cfg)
	require.NoError(t, err)

	// wait for the target to be stopped.
	require.Eventually(t, func() bool {
		return !ta.ready()
	}, 5*time.Second, 100*time.Millisecond)

	require.Len(t, handler.Received(), 0)
	require.GreaterOrEqual(t, calls.Load(), int32(5))
	require.NotEmpty(t, ta.details()["error"])
	ta.stop()
	ps.Stop()

	// Make sure we save the last position.
	newEnd, _ := ps.Get(positions.CursorKey(cfg.ZoneID), cfg.Labels.String())
	require.Equal(t, newEnd, end.UnixNano())
}

func TestTailer_CloudflareTargetError168h(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Inc()
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "bad query: error parsing time: invalid time range: too early: logs older than 168h0m0s are not available")
	}))
	defer srv.Close()

	var (
		logger = logging.NewSlogNop()
		cfg    = &tailerConfig{
			APIToken:   "foo",
			ZoneID:     "bar",
			APIURL:     srv.URL,
			Labels:     model.LabelSet{"job": "cloudflare"},
			PullRange:  model.Duration(time.Minute),
			FieldsType: FieldsTypeDefault,
			Workers:    3,
			Backoff:    backoff.Config{MinBackoff: 0, MaxBackoff: 0, MaxRetries: 5},
		}
		end     = time.Unix(0, time.Hour.Nanoseconds())
		handler = loki.NewCollectingHandler()
	)
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})

	// set our end time to be the last time we have a position
	ps.Put(positions.CursorKey(cfg.ZoneID), cfg.Labels.String(), end.UnixNano())
	require.NoError(t, err)

	ta, err := newTailer(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, cfg)
	require.NoError(t, err)

	// wait for the target to be stopped.
	require.Eventually(t, func() bool {
		return calls.Load() >= 5
	}, 5*time.Second, 100*time.Millisecond)

	require.Len(t, handler.Received(), 0)
	require.GreaterOrEqual(t, calls.Load(), int32(5))
	ta.stop()
	ps.Stop()

	// Make sure we move on from the save the last position.
	newEnd, _ := ps.Get(positions.CursorKey(cfg.ZoneID), cfg.Labels.String())
	require.Greater(t, newEnd, end.UnixNano())
}

func TestTailer_SplitRequests(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  []pullRequest
	}{
		{
			"perfectly divisible",
			time.Unix(0, 0),
			time.Unix(0, int64(time.Minute)),
			[]pullRequest{
				{start: time.Unix(0, 0), end: time.Unix(0, int64(time.Minute/3))},
				{start: time.Unix(0, int64(time.Minute/3)), end: time.Unix(0, int64(time.Minute*2/3))},
				{start: time.Unix(0, int64(time.Minute*2/3)), end: time.Unix(0, int64(time.Minute))},
			},
		},
		{
			"not divisible",
			time.Unix(0, 0),
			time.Unix(0, int64(time.Minute+1)),
			[]pullRequest{
				{start: time.Unix(0, 0), end: time.Unix(0, int64(time.Minute/3))},
				{start: time.Unix(0, int64(time.Minute/3)), end: time.Unix(0, int64(time.Minute*2/3))},
				{start: time.Unix(0, int64(time.Minute*2/3)), end: time.Unix(0, int64(time.Minute+1))},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitRequests(tt.start, tt.end, 3)
			if !assert.Equal(t, tt.want, got) {
				for i := range got {
					if !assert.Equal(t, tt.want[i].start, got[i].start) {
						t.Logf("expected i:%d start: %d , got: %d", i, tt.want[i].start.UnixNano(), got[i].start.UnixNano())
					}
					if !assert.Equal(t, tt.want[i].end, got[i].end) {
						t.Logf("expected i:%d end: %d , got: %d", i, tt.want[i].end.UnixNano(), got[i].end.UnixNano())
					}
				}
			}
		})
	}
}
