package source

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func serve(h http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(h)
}

func doFetch(ctx context.Context, url string, maxSize int64, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	return fetch(http.DefaultClient, req, maxSize)
}

func gzipBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestFetchOK(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("\xef\xbb\xbf[1]")) })
	defer srv.Close()

	body, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.NoError(t, err)
	require.Equal(t, "[1]", string(body))
}

func TestFetchStatus(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	defer srv.Close()

	_, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.Equal(t, reasonStatus, reasonOf(err))
	require.ErrorContains(t, err, "status 503")
}

func TestFetchEmptyBody(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) {})
	defer srv.Close()

	_, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.Equal(t, reasonParse, reasonOf(err))
}

func TestFetchTooLarge(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(bytes.Repeat([]byte("a"), 2048)) })
	defer srv.Close()

	_, err := doFetch(t.Context(), srv.URL, 1024, nil)
	require.Equal(t, reasonTooLarge, reasonOf(err))
}

func TestFetchGzip(t *testing.T) {
	gz, err := gzipBytes([]byte(`{"a":1}`))
	require.NoError(t, err)
	srv := serve(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gz)
	})
	defer srv.Close()

	// An explicit Accept-Encoding stops the transport from decoding, so fetch must do it.
	body, err := doFetch(t.Context(), srv.URL, 1024, http.Header{"Accept-Encoding": {"gzip"}})
	require.NoError(t, err)
	require.Equal(t, `{"a":1}`, string(body))
}

func TestFetchGzipBomb(t *testing.T) {
	gz, err := gzipBytes(make([]byte, 20<<20))
	require.NoError(t, err)
	require.Less(t, len(gz), 1<<20, "the compressed body must be under the limit")
	srv := serve(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gz)
	})
	defer srv.Close()

	_, err = doFetch(t.Context(), srv.URL, 10<<20, http.Header{"Accept-Encoding": {"gzip"}})
	require.Equal(t, reasonTooLarge, reasonOf(err))
}

func TestFetchTimeout(t *testing.T) {
	srv := serve(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := doFetch(ctx, srv.URL, 1024, nil)
	require.Equal(t, reasonTimeout, reasonOf(err))
}

func TestFetchRedactsURL(t *testing.T) {
	srv := serve(func(http.ResponseWriter, *http.Request) {})
	url := srv.URL + "/api?api_key=supersecret"
	srv.Close()

	_, err := doFetch(t.Context(), url, 1024, nil)
	require.Error(t, err)
	require.Equal(t, reasonRequest, reasonOf(err))
	require.NotContains(t, err.Error(), "supersecret")
}

// TestFetchFrameRedactsBuildError checks that an error from building the
// request does not show query params.
func TestFetchFrameRedactsBuildError(t *testing.T) {
	s := querySpec{name: "q", qtype: "json", source: sourceURL, method: http.MethodGet, url: "http://x/%zz?api_key=supersecret"}
	_, err := fetchFrame(t.Context(), &generation{client: http.DefaultClient, timeout: time.Second, maxSize: 1024}, s, atomic.NewInt32(0))
	require.Error(t, err)
	require.Equal(t, reasonRequest, reasonOf(err))
	require.NotContains(t, err.Error(), "supersecret")
}
