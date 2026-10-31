package cloudflare

import (
	"bufio"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

var testBackoff = backoff.Config{MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond, MaxRetries: 4}

func newTestClient(handler http.HandlerFunc, fields []string) (*client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	return newClient(clientConfig{
		apiURL:   srv.URL,
		apiToken: "token",
		zoneID:   "zone-id",
		fields:   fields,
		backoff:  testBackoff,
	}), srv
}

func readAll(t *testing.T, it *logpullIterator) []string {
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
	var (
		start = time.Unix(0, 1000)
		end   = time.Unix(0, 2000)
	)

	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/zones/zone-id/logs/received", r.URL.Path)
		assert.Equal(t, "1000", r.URL.Query().Get("start"))
		assert.Equal(t, "2000", r.URL.Query().Get("end"))
		assert.Equal(t, "ClientIP,EdgeStartTimestamp", r.URL.Query().Get("fields"))
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, "{\"a\":1}\n{\"b\":2}\r\n{\"c\":3}")
	}, []string{"ClientIP", "EdgeStartTimestamp"})
	defer srv.Close()

	it, err := c.LogpullReceived(t.Context(), start, end)
	require.NoError(t, err)
	require.Equal(t, []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}, readAll(t, it))
}

func TestClient_LogpullReceivedGzip(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Accept-Encoding"), "gzip")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = io.WriteString(gz, "{\"a\":1}\n{\"b\":2}\n")
		assert.NoError(t, gz.Close())
	}, nil)
	defer srv.Close()

	it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
	require.NoError(t, err)
	require.Equal(t, []string{`{"a":1}`, `{"b":2}`}, readAll(t, it))
}

func TestClient_LogpullReceivedLongLine(t *testing.T) {
	long := `{"ClientRequestPath":"` + strings.Repeat("a", 100*1024) + `"}`

	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"a":1}`+"\n"+long+"\n"+`{"b":2}`+"\n")
	}, nil)
	defer srv.Close()

	it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
	require.NoError(t, err)
	require.Equal(t, []string{`{"a":1}`, long, `{"b":2}`}, readAll(t, it))
}

func TestLogpullIterator(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		expected []string
	}{
		{name: "empty body", body: "", expected: nil},
		{name: "CRLF line ending", body: "a\r\nb\r\n", expected: []string{"a", "b"}},
		{name: "line over buffer size", body: strings.Repeat("x", 100) + "\nok", expected: []string{strings.Repeat("x", 100), "ok"}},
		{name: "last line without newline", body: "a\nb", expected: []string{"a", "b"}},
		{name: "empty lines are kept", body: "a\n\nb\n", expected: []string{"a", "", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := io.NopCloser(strings.NewReader(tc.body))
			it := &logpullIterator{
				body: body,
				// The smallest buffer bufio allows, so long lines span multiple reads.
				reader: bufio.NewReaderSize(body, 16),
			}
			require.Equal(t, tc.expected, readAll(t, it))
		})
	}
}

func TestLogpullIterator_ReadError(t *testing.T) {
	body := io.NopCloser(io.MultiReader(strings.NewReader("ok\npartial"), errReader{}))
	it := &logpullIterator{body: body, reader: bufio.NewReaderSize(body, 16)}

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
			c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}, nil)
			defer srv.Close()

			it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
			require.Nil(t, it)
			require.EqualError(t, err, tc.expected)
			require.Equal(t, int32(1), calls.Load(), "client errors must not be retried")
		})
	}

	t.Run("too early error is detected", func(t *testing.T) {
		c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "bad query: error parsing time: invalid time range: too early: logs older than 168h0m0s are not available")
		}, nil)
		defer srv.Close()

		_, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
		require.Regexp(t, cloudflareTooEarlyError, err.Error())
	})
}

func TestClient_LogpullReceivedRetries(t *testing.T) {
	t.Run("succeeds after 429 and 5xx responses", func(t *testing.T) {
		var calls atomic.Int32
		c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			switch calls.Add(1) {
			case 1:
				w.WriteHeader(http.StatusTooManyRequests)
			case 2:
				w.WriteHeader(http.StatusBadGateway)
			default:
				_, _ = io.WriteString(w, "{\"a\":1}\n")
			}
		}, nil)
		defer srv.Close()

		it, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
		require.NoError(t, err)
		require.Equal(t, []string{`{"a":1}`}, readAll(t, it))
		require.Equal(t, int32(3), calls.Load())
	})

	t.Run("returns the last error once retries are exhausted", func(t *testing.T) {
		var calls atomic.Int32
		c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "unavailable")
		}, nil)
		defer srv.Close()

		_, err := c.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(0, 1))
		require.EqualError(t, err, "HTTP status 503: unavailable")
		require.Equal(t, int32(4), calls.Load())
	})
}
