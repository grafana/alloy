package scrape

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/util/testlog"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

// captureLoop is a CPU profile loop on server with the given interval.
func captureLoop(t *testing.T, server *httptest.Server, interval time.Duration) *scrapeLoop {
	t.Helper()
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	target := NewTarget(labels.FromStrings(model.MetricNameLabel, pprofProcessCPU, model.SchemeLabel, "http", model.AddressLabel, u.Host, ProfilePath, "/debug/pprof/profile"), url.Values{})
	return newScrapeLoop(target, server.Client(), pyroscope.AppendableFunc(func(context.Context, labels.Labels, []*pyroscope.RawSample) error {
		return nil
	}), interval, time.Second, testlog.TestLogger(t))
}

// startCapture starts the loop as start() does, but captures at once and
// schedules nothing after: upstream's start() waits the target's offset plus a
// full interval before its first capture, and these tests need a capture in
// flight. The capture, its push, retirement and shutdown are the production
// paths (scrape, stop, the pool's retire and stop).
func startCapture(t *scrapeLoop) {
	t.graceShut = make(chan struct{})
	t.once = sync.Once{}
	t.shutdownOnce = sync.Once{}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.scrape()
		<-t.graceShut
	}()
}

var completedProfile = []byte{0x0a, 0x02, 0x6f, 0x6b}

func serveCompleted(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(completedProfile)
}

// Waiting for headers and waiting for a streamed body are separate HTTP states.
// Collector shutdown must cancel both promptly even when the configured capture lasts a minute.
func TestShutdownCancelsActiveCapture(t *testing.T) {
	for _, bodyStarted := range []bool{false, true} {
		name := "headers"
		if bodyStarted {
			name = "body"
		}
		t.Run(name, func(t *testing.T) {
			started := make(chan struct{})
			cancelled := make(chan struct{})
			var appends atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if bodyStarted {
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Write([]byte{0x0a, 0x02})
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
				close(cancelled)
			}))
			defer server.Close()
			defer server.CloseClientConnections()
			loop := captureLoop(t, server, time.Minute)
			loop.timeout = 2 * time.Minute
			loop.appender = pyroscope.AppendableFunc(func(context.Context, labels.Labels, []*pyroscope.RawSample) error { appends.Add(1); return nil }).Appender()
			startCapture(loop)
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("capture never started")
			}
			done := make(chan struct{})
			go func() { loop.stop(true); close(done) }()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("shutdown still waits for the capture timeout")
			}
			select {
			case <-cancelled:
			case <-time.After(time.Second):
				t.Fatal("source request was not cancelled")
			}
			require.Equal(t, int32(0), appends.Load(), "partial capture must not masquerade as a completed profile")
		})
	}
}

// Target removal or replacement retires the loop: the running capture finishes,
// the completed profile is pushed with a live context, and nothing is scheduled afterwards.
func TestRetiredTargetCompletesCaptureAndPush(t *testing.T) {
	started := make(chan struct{})
	var requests, appends atomic.Int32
	var pushErr atomic.Value
	sourceCancelled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(started)
		}
		select {
		case <-r.Context().Done():
			sourceCancelled <- struct{}{}
			return
		case <-time.After(300 * time.Millisecond):
		}
		serveCompleted(w)
	}))
	defer server.Close()
	loop := captureLoop(t, server, 500*time.Millisecond)
	loop.timeout = 2 * time.Minute
	pushed := make(chan struct{}, 4)
	loop.appender = pyroscope.AppendableFunc(func(ctx context.Context, _ labels.Labels, samples []*pyroscope.RawSample) error {
		time.Sleep(200 * time.Millisecond)
		pushErr.Store(ctx.Err() == nil)
		appends.Add(1)
		require.Equal(t, completedProfile, samples[0].RawProfile)
		pushed <- struct{}{}
		return nil
	}).Appender()
	startCapture(loop)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("capture never started")
	}
	loop.stop(false)
	select {
	case <-pushed:
	case <-time.After(3 * time.Second):
		t.Fatal("retired target did not push its completed capture")
	}
	select {
	case <-sourceCancelled:
		t.Fatal("retirement cancelled the running capture")
	default:
	}
	require.Equal(t, true, pushErr.Load(), "retired push context must stay live")
	done := make(chan struct{})
	go func() { loop.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("retired loop kept running")
	}
	require.Equal(t, int32(1), requests.Load(), "retired target scheduled another capture")
	require.Equal(t, int32(1), appends.Load())
}

// A completed profile already inside the writer is never cancelled by retirement,
// even when the writer is slow to accept it.
func TestRetiredTargetPushSurvivesSlowWriter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveCompleted(w) }))
	defer server.Close()
	inWriter := make(chan struct{})
	result := make(chan error, 1)
	loop := captureLoop(t, server, time.Minute)
	loop.appender = pyroscope.AppendableFunc(func(ctx context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
		close(inWriter)
		select {
		case <-ctx.Done():
			result <- ctx.Err()
		case <-time.After(700 * time.Millisecond):
			result <- nil
		}
		return nil
	}).Appender()
	startCapture(loop)
	select {
	case <-inWriter:
	case <-time.After(2 * time.Second):
		t.Fatal("writer never received the completed capture")
	}
	loop.stop(false)
	select {
	case err := <-result:
		require.NoError(t, err, "retirement cancelled a completed profile push")
	case <-time.After(3 * time.Second):
		t.Fatal("writer never finished")
	}
}

// Collector shutdown grants a completed profile the push grace, then cancels it.
func TestShutdownGrantsPushGraceThenCancelsBlockedWriter(t *testing.T) {
	for _, writerDelay := range []time.Duration{100 * time.Millisecond, time.Hour} {
		name := "accepted-within-grace"
		if writerDelay == time.Hour {
			name = "blocked-writer-cancelled-after-grace"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveCompleted(w) }))
			defer server.Close()
			inWriter := make(chan struct{})
			result := make(chan error, 1)
			loop := captureLoop(t, server, time.Minute)
			loop.pushGrace = 400 * time.Millisecond
			loop.appender = pyroscope.AppendableFunc(func(ctx context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
				close(inWriter)
				select {
				case <-ctx.Done():
					result <- ctx.Err()
					return ctx.Err()
				case <-time.After(writerDelay):
					result <- nil
					return nil
				}
			}).Appender()
			startCapture(loop)
			select {
			case <-inWriter:
			case <-time.After(2 * time.Second):
				t.Fatal("writer never received the completed capture")
			}
			began := time.Now()
			done := make(chan struct{})
			go func() { loop.stop(true); close(done) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("shutdown did not bound the writer")
			}
			elapsed := time.Since(began)
			err := <-result
			if writerDelay == time.Hour {
				require.ErrorIs(t, err, context.Canceled)
				require.GreaterOrEqual(t, elapsed, loop.pushGrace, "push cancelled before the grace elapsed")
			} else {
				require.NoError(t, err, "shutdown cancelled a push that would have been accepted within the grace")
				require.Less(t, elapsed, loop.pushGrace, "shutdown waited for the full grace after acceptance")
			}
		})
	}
}

// Retired loops stay bounded by collector shutdown through the pool.
func TestPoolShutdownBoundsRetiredLoops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveCompleted(w) }))
	defer server.Close()
	inWriter := make(chan struct{})
	result := make(chan error, 1)
	loop := captureLoop(t, server, time.Minute)
	loop.pushGrace = 300 * time.Millisecond
	loop.appender = pyroscope.AppendableFunc(func(ctx context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
		close(inWriter)
		<-ctx.Done()
		result <- ctx.Err()
		return ctx.Err()
	}).Appender()
	pool := &scrapePool{activeTargets: map[uint64]*scrapeLoop{}, retired: map[*scrapeLoop]struct{}{}}
	startCapture(loop)
	select {
	case <-inWriter:
	case <-time.After(2 * time.Second):
		t.Fatal("writer never received the completed capture")
	}
	pool.mtx.Lock()
	pool.retire(loop)
	pool.mtx.Unlock()
	pool.mtx.RLock()
	require.Len(t, pool.retired, 1)
	pool.mtx.RUnlock()
	done := make(chan struct{})
	go func() { pool.stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pool shutdown did not bound the retired loop")
	}
	require.ErrorIs(t, <-result, context.Canceled)
	require.Eventually(t, func() bool { pool.mtx.RLock(); defer pool.mtx.RUnlock(); return len(pool.retired) == 0 }, 2*time.Second, 10*time.Millisecond)
}

func TestConcurrentStopIsSafe(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	defer server.CloseClientConnections()
	loop := captureLoop(t, server, time.Minute)
	loop.timeout = 2 * time.Minute
	startCapture(loop)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("capture never started")
	}
	var callers sync.WaitGroup
	for i := 0; i < 16; i++ {
		callers.Add(1)
		go func(i int) { defer callers.Done(); loop.stop(i%2 == 0) }(i)
	}
	done := make(chan struct{})
	go func() { callers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent stop did not finish")
	}
}
