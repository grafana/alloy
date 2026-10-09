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
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// realGetClient keeps the production client constructor, since the tailer
// tests replace getClient with a fake and do not restore it.
var realGetClient = getClient

// logpullRequest is what the stub Logpull API saw in one request.
type logpullRequest struct {
	path, auth, fields string
	start, end         time.Time
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

	var (
		mut      sync.Mutex
		requests []logpullRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		start, _ := strconv.ParseInt(q.Get("start"), 10, 64)
		end, _ := strconv.ParseInt(q.Get("end"), 10, 64)

		mut.Lock()
		requests = append(requests, logpullRequest{
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			fields: q.Get("fields"),
			start:  time.Unix(0, start),
			end:    time.Unix(0, end),
		})
		first := len(requests) == 1
		mut.Unlock()

		// Serve the logs once, so the component cannot pass by re-reading them.
		w.Header().Set("Content-Type", "application/x-ndjson")
		if first {
			_, _ = io.WriteString(w, strings.Join(lines, "\n")+"\n")
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ALLOY_CLOUDFLARE_API_URL", srv.URL)

	ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.cloudflare")
	require.NoError(t, err)

	var (
		receiver  = loki.NewLogsReceiver()
		dataPath  = t.TempDir()
		pullRange = time.Minute
		before    = time.Now()
	)

	ctx, cancel := context.WithCancel(componenttest.TestContext(t))
	runErr := make(chan error, 1)
	go func() {
		runErr <- ctrl.Run(ctx, Arguments{
			APIToken:   alloytypes.Secret("test-token"),
			ZoneID:     "test-zone-id",
			Labels:     map[string]string{"job": "cloudflare"},
			Workers:    1,
			PullRange:  pullRange,
			FieldsType: FieldsTypeDefault,
			ForwardTo:  []loki.LogsReceiver{receiver},
		}, func(opts component.Options) component.Options {
			opts.DataPath = dataPath
			return opts
		})
	}()
	require.NoError(t, ctrl.WaitRunning(time.Minute))

	var received []loki.Entry
	for len(received) < len(lines) {
		select {
		case e := <-receiver.Chan():
			received = append(received, e)
		case <-time.After(10 * time.Second):
			require.FailNow(t, "timed out waiting for log entries", "received %d of %d", len(received), len(lines))
		}
	}

	for i, e := range received {
		require.Equal(t, lines[i], e.Line)
		require.Equal(t, model.LabelSet{"job": "cloudflare"}, e.Labels)
		require.Equal(t, time.Unix(0, int64(1000000001+i)), e.Timestamp, "timestamp comes from EdgeStartTimestamp")
	}

	// No more entries arrive, even if the component pulls again.
	select {
	case e := <-receiver.Chan():
		require.FailNow(t, "unexpected extra entry", e.Line)
	case <-time.After(200 * time.Millisecond):
	}

	after := time.Now()
	cancel()
	require.NoError(t, <-runErr)

	expectedFields, err := fieldsForType(FieldsTypeDefault, nil)
	require.NoError(t, err)

	mut.Lock()
	first := requests[0]
	mut.Unlock()
	require.Equal(t, "/zones/test-zone-id/logs/received", first.path)
	require.Equal(t, "Bearer test-token", first.auth)
	require.Equal(t, strings.Join(expectedFields, ","), first.fields)
	// Cloudflare logs are pulled one pull_range at a time, ending at least a
	// minute in the past.
	require.Equal(t, pullRange, first.end.Sub(first.start))
	require.WithinRange(t, first.end, before.Add(-minDelay), after.Add(-minDelay))

	// The component stores the end of the pulled window as its cursor on
	// shutdown, so a restart resumes from there.
	positions, err := os.ReadFile(filepath.Join(dataPath, "positions.yml"))
	require.NoError(t, err)
	require.Contains(t, string(positions), strconv.FormatInt(first.end.Add(pullRange).UnixNano(), 10))
}
