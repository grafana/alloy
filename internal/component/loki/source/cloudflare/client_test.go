package cloudflare

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func newTestClient(t *testing.T, handler http.HandlerFunc, fields []string) (*wrappedClient, *metrics) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("ALLOY_CLOUDFLARE_API_URL", srv.URL)

	m := newMetrics(prometheus.NewRegistry())
	c, err := newClient("token", "zone-id", fields, logging.NewSlogNop(), m)
	require.NoError(t, err)
	return c.(*wrappedClient), m
}

func readAll(t *testing.T, it LogpullReceivedIterator) []string {
	t.Helper()
	defer it.Close()
	var lines []string
	for it.Next() {
		lines = append(lines, string(it.Line()))
	}
	require.NoError(t, it.Err())
	return lines
}

func TestClient_LogpullReceived(t *testing.T) {
	start := time.Unix(0, 1000)
	end := time.Unix(0, 2000)

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/zones/zone-id/logs/received", r.URL.Path)
		require.Equal(t, "1000", r.URL.Query().Get("start"))
		require.Equal(t, "2000", r.URL.Query().Get("end"))
		require.Equal(t, "ClientIP,EdgeStartTimestamp", r.URL.Query().Get("fields"))
		require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, "{\"a\":1}\n{\"b\":2}\r\n{\"c\":3}")
	}, []string{"ClientIP", "EdgeStartTimestamp"})

	it, err := c.LogpullReceived(t.Context(), start, end)
	require.NoError(t, err)
	require.Equal(t, []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}, readAll(t, it))
}

func TestClient_LogpullReceivedGzip(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Accept-Encoding"), "gzip")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = io.WriteString(gz, "{\"a\":1}\n{\"b\":2}\n")
		require.NoError(t, gz.Close())
	}, nil)

	it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
	require.NoError(t, err)
	require.Equal(t, []string{`{"a":1}`, `{"b":2}`}, readAll(t, it))
}

func TestClient_LogpullReceivedDropsLinesOverMaxSize(t *testing.T) {
	// A 100 KiB line used to fail the whole window with "bufio.Scanner: token too long".
	long := `{"ClientRequestPath":"` + strings.Repeat("a", 100*1024) + `"}`
	tooLong := `{"ClientRequestPath":"` + strings.Repeat("b", maxLineSize) + `"}`

	c, m := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"a":1}`+"\n"+tooLong+"\n"+long+"\n"+tooLong+"\n"+`{"b":2}`+"\n")
	}, nil)

	it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
	require.NoError(t, err)
	require.Equal(t, []string{`{"a":1}`, long, `{"b":2}`}, readAll(t, it))
	require.Equal(t, 2.0, testutil.ToFloat64(m.DroppedLines))
}

func TestLogpullIterator_MaxSize(t *testing.T) {
	const maxSize = 8
	for _, tc := range []struct {
		name     string
		body     string
		expected []string
		dropped  []int
	}{
		{name: "empty body", body: "", expected: nil},
		{name: "line at max size", body: "12345678\n", expected: []string{"12345678"}},
		{name: "line at max size with CRLF", body: "12345678\r\n", expected: []string{"12345678"}},
		{name: "line over max size", body: "123456789\nok\n", expected: []string{"ok"}, dropped: []int{10}},
		{name: "line over buffer size", body: strings.Repeat("x", 100) + "\nok", expected: []string{"ok"}, dropped: []int{101}},
		{name: "last line over max size without newline", body: "ok\n123456789", expected: []string{"ok"}, dropped: []int{9}},
		{name: "empty lines are kept", body: "a\n\nb\n", expected: []string{"a", "", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var dropped []int
			body := io.NopCloser(strings.NewReader(tc.body))
			it := &logpullIterator{
				body: body,
				// The smallest buffer bufio allows, so long lines span multiple reads.
				reader:  bufio.NewReaderSize(body, 16),
				onDrop:  func(size int) { dropped = append(dropped, size) },
				maxSize: maxSize,
			}
			require.Equal(t, tc.expected, readAll(t, it))
			require.Equal(t, tc.dropped, dropped)
		})
	}
}

func TestLogpullIterator_ReadError(t *testing.T) {
	body := io.NopCloser(io.MultiReader(strings.NewReader("ok\npartial"), errReader{}))
	it := &logpullIterator{body: body, reader: bufio.NewReaderSize(body, 16), onDrop: func(int) {}, maxSize: maxLineSize}

	require.True(t, it.Next())
	require.Equal(t, "ok", string(it.Line()))
	require.False(t, it.Next())
	require.ErrorIs(t, it.Err(), io.ErrUnexpectedEOF)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestClient_LogpullReceivedErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		expected string
	}{
		{
			name:     "plain text body",
			status:   http.StatusBadRequest,
			body:     "bad query: error parsing time: invalid time range: too early: logs older than 168h0m0s are not available\n",
			expected: "HTTP status 400: bad query: error parsing time: invalid time range: too early: logs older than 168h0m0s are not available",
		},
		{
			name:     "json body",
			status:   http.StatusForbidden,
			body:     `{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`,
			expected: "HTTP status 403: Authentication error",
		},
		{
			name:     "empty body",
			status:   http.StatusNotFound,
			expected: "HTTP status 404",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}, nil)

			it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
			require.Nil(t, it)
			require.EqualError(t, err, tc.expected)
			require.Equal(t, int32(1), calls.Load(), "client errors must not be retried")
		})
	}

	t.Run("too early error is detected", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "bad query: error parsing time: invalid time range: too early: logs older than 168h0m0s are not available")
		}, nil)

		_, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
		require.Regexp(t, cloudflareTooEarlyError, err.Error())
	})
}

func TestClient_LogpullReceivedRetries(t *testing.T) {
	origBackoff, origRateLimit := requestBackoff, requestRateLimit
	requestBackoff = backoff.Config{MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond, MaxRetries: 4}
	requestRateLimit = rate.Inf
	t.Cleanup(func() { requestBackoff, requestRateLimit = origBackoff, origRateLimit })

	t.Run("succeeds after 429 and 5xx responses", func(t *testing.T) {
		var calls atomic.Int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch calls.Add(1) {
			case 1:
				w.WriteHeader(http.StatusTooManyRequests)
			case 2:
				w.WriteHeader(http.StatusBadGateway)
			default:
				_, _ = io.WriteString(w, "{\"a\":1}\n")
			}
		}, nil)

		it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
		require.NoError(t, err)
		require.Equal(t, []string{`{"a":1}`}, readAll(t, it))
		require.Equal(t, int32(3), calls.Load())
	})

	t.Run("returns the last error once retries are exhausted", func(t *testing.T) {
		var calls atomic.Int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "unavailable")
		}, nil)

		_, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
		require.EqualError(t, err, "HTTP status 503: unavailable")
		require.Equal(t, int32(4), calls.Load())
	})
}

// TestTailer_DropsLinesOverMaxSize checks that a pull window containing a line over the
// maximum line size still completes and forwards the other lines.
func TestClient_LogpullReceivedRateLimit(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {}, nil)
	c.rateLimiter = rate.NewLimiter(rate.Every(time.Hour), 1)

	it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
	require.NoError(t, err)
	require.NoError(t, it.Close())

	// The second request has to wait for the rate limiter, which gives up because
	// the wait would exceed the context deadline.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err = c.LogpullReceived(ctx, time.Unix(0, 0), time.Unix(0, 1))
	require.Error(t, err)
}

func TestTailer_DropsLinesOverMaxSize(t *testing.T) {
	tooLong := `{"EdgeStartTimestamp":1,"ClientRequestPath":"` + strings.Repeat("a", maxLineSize) + `"}`
	var served atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if served.Swap(true) {
			return
		}
		_, _ = fmt.Fprintf(w, "%s\n%s\n", tooLong, `{"EdgeStartTimestamp":2}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ALLOY_CLOUDFLARE_API_URL", srv.URL)

	origGetClient := getClient
	getClient = newClient
	t.Cleanup(func() { getClient = origGetClient })

	var (
		logger = logging.NewSlogNop()
		cfg    = &tailerConfig{
			APIToken:   "foo",
			ZoneID:     "bar",
			Labels:     model.LabelSet{"job": "cloudflare"},
			PullRange:  model.Duration(time.Minute),
			FieldsType: FieldsTypeDefault,
			Workers:    1,
			Backoff:    backoff.Config{MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond, MaxRetries: 1},
		}
		handler = loki.NewCollectingHandler()
		m       = newMetrics(prometheus.NewRegistry())
	)
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)
	end := time.Now().Add(-2 * time.Minute)
	ps.Put(positions.CursorKey(cfg.ZoneID), cfg.Labels.String(), end.UnixNano())

	ta, err := newTailer(m, logger, handler.Receiver(), ps, cfg)
	require.NoError(t, err)
	defer ps.Stop()

	require.Eventually(t, func() bool {
		return len(handler.Received()) == 1
	}, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, `{"EdgeStartTimestamp":2}`, handler.Received()[0].Line)
	require.Equal(t, 1.0, testutil.ToFloat64(m.DroppedLines))

	ta.stop()
	require.NoError(t, ta.err)
}
