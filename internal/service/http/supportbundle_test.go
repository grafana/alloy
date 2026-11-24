package http

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetrieveEnvironmentVariables_RedactsProxies(t *testing.T) {
	env := map[string]string{
		"HTTP_PROXY":  "http://user:secret@proxy.example.com:3128",
		"https_proxy": "http://user:secret@proxy.example.com:3128",
		"NO_PROXY":    "localhost",
	}
	getenv := func(key string) string { return env[key] }

	values := retrieveEnvironmentVariables(getenv)

	require.Contains(t, values, "HTTP_PROXY=<redacted>")
	require.Contains(t, values, "https_proxy=<redacted>")
	require.Contains(t, values, "HTTPS_PROXY=")
	require.Contains(t, values, "NO_PROXY=localhost")
	for _, v := range values {
		require.NotContains(t, v, "secret")
	}
}

type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error {
	c.closed = true
	return nil
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRetrieveAPIEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte("hello"))
		case "/created":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("made"))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
		}
	}))
	defer srv.Close()
	addr := srv.Listener.Addr().(*net.TCPAddr).String()

	t.Run("2xx returns body", func(t *testing.T) {
		for path, want := range map[string]string{"ok": "hello", "created": "made"} {
			got, err := retrieveAPIEndpoint(*srv.Client(), addr, path)
			require.NoError(t, err)
			require.Equal(t, want, string(got))
		}
	})

	t.Run("non-2xx returns error with endpoint and status", func(t *testing.T) {
		got, err := retrieveAPIEndpoint(*srv.Client(), addr, "metrics")
		require.Error(t, err)
		require.Nil(t, got)
		require.Contains(t, err.Error(), "/metrics")
		require.Contains(t, err.Error(), "503")
	})

	t.Run("body closed on non-2xx", func(t *testing.T) {
		body := &closeTracker{Reader: strings.NewReader("x")}
		client := http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 503, Status: "503 Service Unavailable", Body: body}, nil
		})}
		_, err := retrieveAPIEndpoint(client, addr, "metrics")
		require.Error(t, err)
		require.True(t, body.closed)
	})

	t.Run("read failure returns error and closes body", func(t *testing.T) {
		body := &closeTracker{Reader: errReader{}}
		client := http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Status: "200 OK", Body: body}, nil
		})}
		_, err := retrieveAPIEndpoint(client, addr, "metrics")
		require.ErrorContains(t, err, "read boom")
		require.True(t, body.closed)
	})
}
