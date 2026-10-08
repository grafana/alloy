package cloudflare

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realGetClient keeps the production client constructor, since other tests in
// this package replace getClient with a fake and do not restore it.
var realGetClient = getClient

// TestClient_LogpullReceived drives the real Cloudflare client against a stub
// Logpull API, covering the request it sends and how it reads the NDJSON
// response. The tailer tests use a fake client, so they cover neither.
func TestClient_LogpullReceived(t *testing.T) {
	lines := []string{
		`{"EdgeStartTimestamp":1,"ClientIP":"192.168.0.1","ClientRequestMethod":"GET","EdgeResponseStatus":200,"RayID":"test-ray-001"}`,
		`{"EdgeStartTimestamp":2,"ClientIP":"10.0.0.2","ClientRequestMethod":"POST","EdgeResponseStatus":201,"RayID":"test-ray-002"}`,
		`{"EdgeStartTimestamp":3,"ClientIP":"172.16.0.3","ClientRequestMethod":"GET","EdgeResponseStatus":200,"RayID":"test-ray-003"}`,
	}

	var (
		start  = time.Unix(0, 1_000_000_000)
		end    = start.Add(time.Minute)
		fields = []string{"ClientIP", "EdgeStartTimestamp", "RayID"}
	)

	for _, gzipped := range []bool{false, true} {
		t.Run("gzip="+strconv.FormatBool(gzipped), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/zones/test-zone-id/logs/received", r.URL.Path)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				assert.Equal(t, "gzip", r.Header.Get("Accept-Encoding"))

				q := r.URL.Query()
				assert.Equal(t, strconv.FormatInt(start.UnixNano(), 10), q.Get("start"))
				assert.Equal(t, strconv.FormatInt(end.UnixNano(), 10), q.Get("end"))
				assert.Equal(t, strings.Join(fields, ","), q.Get("fields"))

				body := strings.Join(lines, "\n") + "\n"
				w.Header().Set("Content-Type", "application/x-ndjson")
				if !gzipped {
					_, _ = io.WriteString(w, body)
					return
				}
				w.Header().Set("Content-Encoding", "gzip")
				gz := gzip.NewWriter(w)
				_, _ = io.WriteString(gz, body)
				_ = gz.Close()
			}))
			t.Cleanup(srv.Close)
			t.Setenv("ALLOY_CLOUDFLARE_API_URL", srv.URL)

			client, err := realGetClient("test-token", "test-zone-id", fields)
			require.NoError(t, err)

			it, err := client.LogpullReceived(t.Context(), start, end)
			require.NoError(t, err)
			defer it.Close()

			var got []string
			for it.Next() {
				got = append(got, string(it.Line()))
			}
			require.NoError(t, it.Err())
			require.Equal(t, lines, got)
		})
	}
}

func TestClient_LogpullReceivedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":10000,"message":"Authentication error"}]}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ALLOY_CLOUDFLARE_API_URL", srv.URL)

	client, err := realGetClient("bad-token", "test-zone-id", nil)
	require.NoError(t, err)

	_, err = client.LogpullReceived(t.Context(), time.Unix(0, 0), time.Unix(60, 0))
	require.ErrorContains(t, err, "Authentication error")
}
