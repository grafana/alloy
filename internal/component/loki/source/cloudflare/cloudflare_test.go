package cloudflare

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// realGetClient keeps the production client constructor, since the tailer
// tests replace getClient with a fake and do not restore it.
var realGetClient = getClient

// logpullRequest is what a stub Logpull API saw in one request.
type logpullRequest struct {
	path, auth, fields string
	start, end         time.Time
}

// logpullStub is a stub Cloudflare API. It serves lines on its first Logpull
// request only, so a component cannot pass by reading them again.
type logpullStub struct {
	*httptest.Server

	mut      sync.Mutex
	requests []logpullRequest
}

func newLogpullStub(t *testing.T, lines ...string) *logpullStub {
	s := &logpullStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		start, _ := strconv.ParseInt(q.Get("start"), 10, 64)
		end, _ := strconv.ParseInt(q.Get("end"), 10, 64)

		s.mut.Lock()
		s.requests = append(s.requests, logpullRequest{
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			fields: q.Get("fields"),
			start:  time.Unix(0, start),
			end:    time.Unix(0, end),
		})
		first := len(s.requests) == 1
		s.mut.Unlock()

		w.Header().Set("Content-Type", "application/x-ndjson")
		if first {
			_, _ = io.WriteString(w, strings.Join(lines, "\n")+"\n")
		}
	}))
	t.Cleanup(s.Close)
	return s
}

// apiURL mirrors the path layout of the real API, so the test also checks
// that the configured path prefix is kept.
func (s *logpullStub) apiURL() string { return s.URL + "/client/v4" }

func (s *logpullStub) firstRequest(t *testing.T) logpullRequest {
	s.mut.Lock()
	defer s.mut.Unlock()
	require.NotEmpty(t, s.requests, "stub at %s received no requests", s.URL)
	return s.requests[0]
}

// TestComponent runs the whole component against a stub Logpull API. The
// tailer tests cover the pull loop with a fake client, so this is the test that
// covers the component and the real client together.
func TestComponent(t *testing.T) {
	getClient = realGetClient
	t.Cleanup(func() { getClient = realGetClient })

	lines := []string{
		`{"EdgeStartTimestamp":1000000001,"ClientIP":"192.168.0.1","ClientRequestMethod":"GET","EdgeResponseStatus":200,"RayID":"test-ray-001"}`,
		`{"EdgeStartTimestamp":1000000002,"ClientIP":"10.0.0.2","ClientRequestMethod":"POST","EdgeResponseStatus":201,"RayID":"test-ray-002"}`,
		`{"EdgeStartTimestamp":1000000003,"ClientIP":"172.16.0.3","ClientRequestMethod":"GET","EdgeResponseStatus":200,"RayID":"test-ray-003"}`,
	}
	reloaded := `{"EdgeStartTimestamp":1000000004,"ClientIP":"10.0.0.4","ClientRequestMethod":"GET","EdgeResponseStatus":200,"RayID":"test-ray-004"}`

	stub := newLogpullStub(t, lines...)
	reloadStub := newLogpullStub(t, reloaded)

	ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.cloudflare")
	require.NoError(t, err)

	var (
		receiver  = loki.NewLogsReceiver()
		dataPath  = t.TempDir()
		pullRange = time.Minute
		before    = time.Now()
		args      = Arguments{
			APIToken:   alloytypes.Secret("test-token"),
			ZoneID:     "test-zone-id",
			APIURL:     stub.apiURL(),
			Labels:     map[string]string{"job": "cloudflare"},
			Workers:    1,
			PullRange:  pullRange,
			FieldsType: FieldsTypeDefault,
			ForwardTo:  []loki.LogsReceiver{receiver},
		}
	)

	ctx, cancel := context.WithCancel(componenttest.TestContext(t))
	runErr := make(chan error, 1)
	go func() {
		runErr <- ctrl.Run(ctx, args, func(opts component.Options) component.Options {
			opts.DataPath = dataPath
			return opts
		})
	}()
	require.NoError(t, ctrl.WaitRunning(time.Minute))

	received := receive(t, receiver, len(lines))
	for i, e := range received {
		require.Equal(t, lines[i], e.Line)
		require.Equal(t, model.LabelSet{"job": "cloudflare"}, e.Labels)
		require.Equal(t, time.Unix(0, int64(1000000001+i)), e.Timestamp, "timestamp comes from EdgeStartTimestamp")
	}
	after := time.Now()

	expectedFields, err := fieldsForType(FieldsTypeDefault, nil)
	require.NoError(t, err)

	first := stub.firstRequest(t)
	require.Equal(t, "/client/v4/zones/test-zone-id/logs/received", first.path)
	require.Equal(t, "Bearer test-token", first.auth)
	require.Equal(t, strings.Join(expectedFields, ","), first.fields)
	// Cloudflare logs are pulled one pull_range at a time, ending at least a
	// minute in the past.
	require.Equal(t, pullRange, first.end.Sub(first.start))
	require.WithinRange(t, first.end, before.Add(-minDelay), after.Add(-minDelay))

	// Changing api_url on a config reload moves the component to the new API.
	args.APIURL = reloadStub.apiURL()
	require.NoError(t, ctrl.Update(args))
	require.Equal(t, reloaded, receive(t, receiver, 1)[0].Line)
	require.Equal(t, "/client/v4/zones/test-zone-id/logs/received", reloadStub.firstRequest(t).path)

	// No more entries arrive, even if the component pulls again.
	select {
	case e := <-receiver.Chan():
		require.FailNow(t, "unexpected extra entry", e.Line)
	case <-time.After(200 * time.Millisecond):
	}

	cancel()
	require.NoError(t, <-runErr)

	// The component stores the end of the pulled window as its cursor on
	// shutdown, so a restart resumes from there.
	stored, err := os.ReadFile(filepath.Join(dataPath, "positions.yml"))
	require.NoError(t, err)
	require.Contains(t, string(stored), positions.CursorKey("test-zone-id"))
}

func receive(t *testing.T, receiver loki.LogsReceiver, n int) []loki.Entry {
	t.Helper()
	var received []loki.Entry
	for len(received) < n {
		select {
		case e := <-receiver.Chan():
			received = append(received, e)
		case <-time.After(10 * time.Second):
			require.FailNow(t, "timed out waiting for log entries", "received %d of %d", len(received), n)
		}
	}
	return received
}

func TestArguments(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		wantAPIURL string
		wantErr    string
	}{
		{
			name:       "api_url defaults to the Cloudflare API",
			config:     `api_token = "t"` + "\n" + `zone_id = "z"` + "\n" + `forward_to = []`,
			wantAPIURL: "https://api.cloudflare.com/client/v4",
		},
		{
			name:       "api_url can be set",
			config:     `api_token = "t"` + "\n" + `zone_id = "z"` + "\n" + `api_url = "http://localhost:8080/client/v4"` + "\n" + `forward_to = []`,
			wantAPIURL: "http://localhost:8080/client/v4",
		},
		{
			name:    "api_url must be absolute",
			config:  `api_token = "t"` + "\n" + `zone_id = "z"` + "\n" + `api_url = "/client/v4"` + "\n" + `forward_to = []`,
			wantErr: "api_url must be an absolute http or https URL",
		},
		{
			name:    "api_url must be http or https",
			config:  `api_token = "t"` + "\n" + `zone_id = "z"` + "\n" + `api_url = "ftp://example.com"` + "\n" + `forward_to = []`,
			wantErr: "api_url must be an absolute http or https URL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.config), &args)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantAPIURL, args.APIURL)
		})
	}
}

func TestTailerConfigTrimsAPIURL(t *testing.T) {
	args := DefaultArguments
	args.APIURL = "https://example.com/client/v4/"
	require.Equal(t, "https://example.com/client/v4", args.tailerConfig().APIURL)
}
