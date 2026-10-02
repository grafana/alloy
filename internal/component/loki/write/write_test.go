package write

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	lokiutil "github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestWriteToSingleEndpoint(t *testing.T) {
	runWriteTest(t, func(t *testing.T, write writeFunc) {
		t.Run("wal disabled", func(t *testing.T) {
			testSingleEndpoint(t, write, func(args *Arguments) {})
		})

		t.Run("wal enabled", func(t *testing.T) {
			testSingleEndpoint(t, write, func(args *Arguments) {
				args.WAL.Enabled = true
			})
		})
	})
}

func testSingleEndpoint(t *testing.T, write writeFunc, alterConfig func(arguments *Arguments)) {
	received := make(chan lokiutil.RemoteWriteRequest, 100)
	srv := lokiutil.NewRemoteWriteServer(received, http.StatusOK)
	defer srv.Close()

	// Set up the component Arguments.
	cfg := fmt.Sprintf(`
		endpoint {
			url        = "%s"
			batch_wait = "10ms"
			tenant_id  = "tenant-1"
		}
	`, srv.URL)
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	alterConfig(&args)

	// Set up and start the component.
	tc, err := componenttest.NewControllerFromID(slog.New(slog.DiscardHandler), "loki.write")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()
	require.NoError(t, tc.WaitExports(time.Second))
	require.NoError(t, tc.WaitRunning(time.Second))

	// Send two log entries to the component's receiver
	logEntry := loki.Entry{
		Labels: model.LabelSet{"foo": "bar"},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "very important log",
		},
	}

	require.NoError(t, write(t.Context(), []loki.Entry{logEntry, logEntry}, tc))

	// Wait for our exporter to finish and pass data to our HTTP server.
	// Make sure the log entries were received correctly.
	entries := []push.Entry{}
LOOP:
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			break LOOP
		case req := <-received:
			require.Equal(t, "tenant-1", req.TenantID)
			require.Len(t, req.Request.Streams, 1)
			require.Equal(t, req.Request.Streams[0].Labels, logEntry.Labels.String())
			entries = append(entries, req.Request.Streams[0].Entries...)
		}
	}
	require.Len(t, entries, 2)
	require.Equal(t, entries[0].Line, logEntry.Entry.Line)
	require.Equal(t, entries[1].Line, logEntry.Entry.Line)
}

func TestEntrySentToTwoWriteComponents(t *testing.T) {
	runWriteTest(t, func(t *testing.T, write writeFunc) {
		t.Run("wal disabled", func(t *testing.T) {
			testMultipleEndpoint(t, write, func(arguments *Arguments) {})
		})

		t.Run("wal enabled", func(t *testing.T) {
			testMultipleEndpoint(t, write, func(arguments *Arguments) {
				arguments.WAL.Enabled = true
			})
		})
	})
}

func testMultipleEndpoint(t *testing.T, write writeFunc, alterArgs func(arguments *Arguments)) {
	ch1, ch2 := make(chan lokiutil.RemoteWriteRequest, 100), make(chan lokiutil.RemoteWriteRequest, 100)
	srv1 := lokiutil.NewRemoteWriteServer(ch1, http.StatusOK)
	srv2 := lokiutil.NewRemoteWriteServer(ch2, http.StatusOK)
	defer srv1.Close()
	defer srv2.Close()

	// Set up two different loki.write components.
	cfg1 := fmt.Sprintf(`
		endpoint {
			url        = "%s"
		}
		external_labels = { "lbl" = "foo" }
	`, srv1.URL)
	cfg2 := fmt.Sprintf(`
		endpoint {
			url        = "%s"
		}
		external_labels = { "lbl" = "bar" }
	`, srv2.URL)
	var args1, args2 Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg1), &args1))
	require.NoError(t, syntax.Unmarshal([]byte(cfg2), &args2))
	alterArgs(&args1)
	alterArgs(&args2)

	// Set up and start the components.
	ctrl1, err := componenttest.NewControllerFromID(slog.New(slog.DiscardHandler), "loki.write")
	require.NoError(t, err)
	go func() {
		require.NoError(t, ctrl1.Run(componenttest.TestContext(t), args1))
	}()
	require.NoError(t, ctrl1.WaitExports(time.Second))
	require.NoError(t, ctrl1.WaitRunning(time.Second))

	ctrl2, err := componenttest.NewControllerFromID(slog.New(slog.DiscardHandler), "loki.write")
	require.NoError(t, err)
	go func() {
		require.NoError(t, ctrl2.Run(componenttest.TestContext(t), args2))
	}()
	require.NoError(t, ctrl2.WaitExports(time.Second))
	require.NoError(t, ctrl2.WaitRunning(time.Second))

	wantLabelSet := model.LabelSet{"somelbl": "somevalue"}
	require.NoError(
		t,
		write(
			t.Context(),
			[]loki.Entry{
				loki.NewEntry(wantLabelSet, push.Entry{
					Timestamp: time.Now(),
					Line:      "writing some text",
				}),
			},
			ctrl1,
			ctrl2,
		),
	)

	// Each endpoint receives the entry with its component's external labels merged in.
	for i := 0; i < 2; i++ {
		select {
		case <-time.After(2 * time.Second):
			require.FailNow(t, "failed waiting for logs")
		case req := <-ch1:
			require.Len(t, req.Request.Streams, 1)
			require.Equal(t, wantLabelSet.Clone().Merge(model.LabelSet{"lbl": "foo"}).String(), req.Request.Streams[0].Labels)
			require.Len(t, req.Request.Streams[0].Entries, 1)
			require.Equal(t, "writing some text", req.Request.Streams[0].Entries[0].Line)
		case req := <-ch2:
			require.Len(t, req.Request.Streams, 1)
			require.Equal(t, wantLabelSet.Clone().Merge(model.LabelSet{"lbl": "bar"}).String(), req.Request.Streams[0].Labels)
			require.Len(t, req.Request.Streams[0].Entries, 1)
			require.Equal(t, "writing some text", req.Request.Streams[0].Entries[0].Line)
		}
	}
}

func TestComponentExperimentalConfig(t *testing.T) {
	t.Run("should be able to create component with default queue_config", func(t *testing.T) {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			endpoint {
				url = "test.com"
			}
		`), &args)
		require.NoError(t, err)

		_, err = New(component.Options{
			Logger:        logging.NewSlogNop(),
			MinStability:  featuregate.StabilityGenerallyAvailable,
			OnStateChange: func(e component.Exports) {},
		}, args)
		require.NoError(t, err)
	})

	t.Run("should not be able to create component with experimental config without required stability level", func(t *testing.T) {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			endpoint {
				url = "test.com"
				queue_config {
					min_shards = 2
				}	
			}
		`), &args)
		require.NoError(t, err)

		_, err = New(component.Options{
			Logger:        logging.NewSlogNop(),
			MinStability:  featuregate.StabilityGenerallyAvailable,
			OnStateChange: func(e component.Exports) {},
		}, args)

		require.Error(t, err)
	})

	t.Run("should be able to create component with experimental config with required stability level", func(t *testing.T) {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			endpoint {
				url = "test.com"
				queue_config {
					min_shards = 2
				}	
			}
		`), &args)
		require.NoError(t, err)

		_, err = New(component.Options{
			Logger:        logging.NewSlogNop(),
			MinStability:  featuregate.StabilityExperimental,
			OnStateChange: func(e component.Exports) {},
		}, args)

		require.NoError(t, err)
	})

	t.Run("should be able to create component with disabled wal without required stability level", func(t *testing.T) {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			endpoint {
				url = "test.com"
			}
			wal {
				enabled = false
			}
		`), &args)
		require.NoError(t, err)

		_, err = New(component.Options{
			Logger:        logging.NewSlogNop(),
			MinStability:  featuregate.StabilityGenerallyAvailable,
			OnStateChange: func(e component.Exports) {},
		}, args)
		require.NoError(t, err)
	})

	t.Run("should not be able to create component with enabled wal without required stability level", func(t *testing.T) {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			endpoint {
				url = "test.com"
			}
			wal {
				enabled = true
			}
		`), &args)
		require.NoError(t, err)

		_, err = New(component.Options{
			Logger:        logging.NewSlogNop(),
			MinStability:  featuregate.StabilityGenerallyAvailable,
			OnStateChange: func(e component.Exports) {},
		}, args)
		require.ErrorContains(t, err, "enabling wal requires stability.level flag to be experimental")
	})

	t.Run("should be able to create component with enabled wal with required stability level", func(t *testing.T) {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			endpoint {
				url = "test.com"
			}
			wal {
				enabled = true
			}
		`), &args)
		require.NoError(t, err)

		_, err = New(component.Options{
			Logger:        logging.NewSlogNop(),
			MinStability:  featuregate.StabilityExperimental,
			DataPath:      t.TempDir(),
			OnStateChange: func(e component.Exports) {},
		}, args)
		require.NoError(t, err)
	})
}

func BenchmarkLokiWrite(b *testing.B) {
	type testCase struct {
		name       string
		numEntries int
		numSeries  int
	}

	tests := []testCase{
		{
			name:       "100 lines, single series",
			numEntries: 100,
			numSeries:  1,
		},
		{
			name:       "100k lines, 100 series",
			numEntries: 100_000,
			numSeries:  100,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.Run("Receiver", func(b *testing.B) { benchSingleEndpoint(b, tt.numSeries, tt.numEntries, writeReceiver) })
			b.Run("Consume", func(b *testing.B) { benchSingleEndpoint(b, tt.numSeries, tt.numEntries, writeConsume) })
		})
	}
}

func benchSingleEndpoint(b *testing.B, numSeries, numEntries int, write writeFunc) {
	// Set up the server that will receive the log entry, and expose it on ch.
	var seenLines atomic.Int64
	ch := make(chan lokiutil.RemoteWriteRequest)

	// just count seenLines for each entry received
	go func() {
		for req := range ch {
			count := 0
			for _, str := range req.Request.Streams {
				count += len(str.Entries)
			}
			seenLines.Add(int64(count))
		}
	}()

	srv := lokiutil.NewRemoteWriteServer(ch, http.StatusOK)
	defer srv.Close()

	// Set up the component Arguments.
	cfg := fmt.Sprintf(`
		endpoint {
			url        = "%s"
			batch_wait = "10ms"
			tenant_id  = "tenant-1"
		}
	`, srv.URL)
	var args Arguments
	require.NoError(b, syntax.Unmarshal([]byte(cfg), &args))

	// Set up and start the component.
	testComp, err := componenttest.NewControllerFromID(util.TestLogger(b), "loki.write")
	require.NoError(b, err)
	go func() {
		err = testComp.Run(componenttest.TestContext(b), args)
		require.NoError(b, err)
	}()
	require.NoError(b, testComp.WaitExports(time.Minute))
	require.NoError(b, testComp.WaitRunning(time.Minute))

	entries := make([]loki.Entry, 0, numEntries)
	for i := 0; i < numEntries; i++ {
		entries = append(entries, loki.Entry{
			Labels: model.LabelSet{"foo": model.LabelValue(fmt.Sprintf("bar-%d", i%numSeries))},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "very important log",
			},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, write(b.Context(), entries, testComp))
		require.Eventually(b, func() bool {
			return int64((i+1)*numEntries) == seenLines.Load()
		}, time.Minute, 100*time.Millisecond, "haven't seen expected number of lines")
	}
}

func TestUpdateWhileSendingIsBlocked(t *testing.T) {
	runWriteTest(t, func(t *testing.T, write writeFunc) {
		for _, walEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("wal_enabled=%t", walEnabled), func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				ctrl, _ := blockedComponent(t, ctx, walEnabled, write)

				updated := make(chan error, 1)
				go func() { updated <- ctrl.Update(blockedEndpointArgs(t, "http://localhost:1", walEnabled)) }()

				select {
				case err := <-updated:
					require.NoError(t, err)
				case <-time.After(30 * time.Second):
					t.Fatal("Update did not return while sending was blocked")
				}
			})
		}
	})
}

func TestStopWhileSendingIsBlocked(t *testing.T) {
	runWriteTest(t, func(t *testing.T, write writeFunc) {
		for _, walEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("wal_enabled=%t", walEnabled), func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				_, stopped := blockedComponent(t, ctx, walEnabled, write)
				cancel()

				select {
				case err := <-stopped:
					require.NoError(t, err)
				case <-time.After(30 * time.Second):
					t.Fatal("Run did not return while sending was blocked")
				}
			})
		}
	})
}

func blockedComponent(t *testing.T, ctx context.Context, walEnabled bool, write writeFunc) (*componenttest.Controller, <-chan error) {
	t.Helper()

	blocked := atomic.NewBool(false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blocked.Store(true)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	ctrl, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.write")
	require.NoError(t, err)

	stopped := make(chan error, 1)
	go func() {
		stopped <- ctrl.Run(ctx, blockedEndpointArgs(t, srv.URL, walEnabled), func(o component.Options) component.Options {
			o.MinStability = featuregate.StabilityExperimental
			return o
		})
	}()

	require.NoError(t, ctrl.WaitExports(5*time.Second))

	go func() {
		entry := loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "very important log"})
		// Writes block once the queue is full. Once the component is stopped a
		// Consume write returns right away without an error, so ctx is checked
		// to stop.
		for ctx.Err() == nil {
			_ = write(ctx, []loki.Entry{entry}, ctrl)
		}
	}()

	require.Eventually(t, blocked.Load, 10*time.Second, 10*time.Millisecond)
	return ctrl, stopped
}
func blockedEndpointArgs(t *testing.T, url string, walEnabled bool) Arguments {
	t.Helper()

	cfg := fmt.Sprintf(`
		endpoint {
			url                 = "%s"
			batch_size          = "1B"
			min_backoff_period  = "1ms"
			max_backoff_period  = "5ms"
			max_backoff_retries = 0

			queue_config {
				capacity          = "1B"
				min_shards        = 1
				drain_timeout     = "1s"
				block_on_overflow = true
			}
		}

		wal {
			enabled       = %t
			drain_timeout = "1s"
		}
	`, url, walEnabled)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	return args
}

func TestFailedUpdateKeepsPreviousConsumer(t *testing.T) {
	runWriteTest(t, func(t *testing.T, write writeFunc) {
		for _, walEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("wal_enabled=%t", walEnabled), func(t *testing.T) {
				firstReceived := make(chan lokiutil.RemoteWriteRequest, 100)
				firstSrv := lokiutil.NewRemoteWriteServer(firstReceived, http.StatusOK)
				defer firstSrv.Close()

				secondReceived := make(chan lokiutil.RemoteWriteRequest, 100)
				secondSrv := lokiutil.NewRemoteWriteServer(secondReceived, http.StatusOK)
				defer secondSrv.Close()

				firstEndpoint := fmt.Sprintf(`
					endpoint {
						url        = "%s"
						batch_wait = "10ms"
					}
				`, firstSrv.URL)

				secondEndpoint := fmt.Sprintf(`
					endpoint {
						url        = "%s"
						batch_wait = "10ms"
					}
				`, secondSrv.URL)

				var firstArgs Arguments
				require.NoError(t, syntax.Unmarshal([]byte(firstEndpoint), &firstArgs))
				firstArgs.WAL.Enabled = walEnabled

				// Moves to the second server, but both endpoints hash to the same name so
				// building the consumer fails.
				var failingArgs Arguments
				require.NoError(t, syntax.Unmarshal([]byte(secondEndpoint+secondEndpoint), &failingArgs))
				failingArgs.WAL.Enabled = walEnabled

				var secondArgs Arguments
				require.NoError(t, syntax.Unmarshal([]byte(secondEndpoint), &secondArgs))
				secondArgs.WAL.Enabled = walEnabled

				ctrl, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.write")
				require.NoError(t, err)

				go func() { require.NoError(t, ctrl.Run(componenttest.TestContext(t), firstArgs)) }()
				require.NoError(t, ctrl.WaitExports(5*time.Second))
				require.NoError(t, ctrl.WaitRunning(5*time.Second))

				require.NoError(t, write(t.Context(), []loki.Entry{loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "first"})}, ctrl))
				waitForLine(t, firstReceived, "first")

				require.Error(t, ctrl.Update(failingArgs))

				// The failed update left the previous consumer in place.
				require.NoError(t, write(t.Context(), []loki.Entry{loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "second"})}, ctrl))
				waitForLine(t, firstReceived, "second")

				require.NoError(t, ctrl.Update(secondArgs))

				require.NoError(t, write(t.Context(), []loki.Entry{loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "third"})}, ctrl))
				waitForLine(t, secondReceived, "third")
			})
		}
	})
}

func TestUpdateTogglesWAL(t *testing.T) {
	runWriteTest(t, func(t *testing.T, write writeFunc) {
		received := make(chan lokiutil.RemoteWriteRequest, 100)
		srv := lokiutil.NewRemoteWriteServer(received, http.StatusOK)
		defer srv.Close()

		endpoint := fmt.Sprintf(`
			endpoint {
				url        = "%s"
				batch_wait = "10ms"
			}
		`, srv.URL)

		var walArgs Arguments
		require.NoError(t, syntax.Unmarshal([]byte(endpoint), &walArgs))
		walArgs.WAL.Enabled = true

		var noWalArgs Arguments
		require.NoError(t, syntax.Unmarshal([]byte(endpoint), &noWalArgs))

		ctrl, err := componenttest.NewControllerFromID(logging.NewSlogNop(), "loki.write")
		require.NoError(t, err)

		go func() { require.NoError(t, ctrl.Run(componenttest.TestContext(t), walArgs)) }()
		require.NoError(t, ctrl.WaitExports(5*time.Second))
		require.NoError(t, ctrl.WaitRunning(5*time.Second))

		require.NoError(t, write(t.Context(), []loki.Entry{loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "with wal"})}, ctrl))
		waitForLine(t, received, "with wal")

		require.NoError(t, ctrl.Update(noWalArgs))

		require.NoError(t, write(t.Context(), []loki.Entry{loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "without wal"})}, ctrl))
		waitForLine(t, received, "without wal")

		require.NoError(t, ctrl.Update(walArgs))

		require.NoError(t, write(t.Context(), []loki.Entry{loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "with wal again"})}, ctrl))
		waitForLine(t, received, "with wal again")
	})
}

// waitForLine waits until an entry with the wanted line is received. Lines are matched
// instead of asserting on the next request, a reload can re-deliver entries that the
// previous consumer had already sent.
func waitForLine(t *testing.T, received chan lokiutil.RemoteWriteRequest, want string) {
	t.Helper()

	timeout := time.After(10 * time.Second)
	for {
		select {
		case req := <-received:
			for _, stream := range req.Request.Streams {
				for _, entry := range stream.Entries {
					if entry.Line == want {
						return
					}
				}
			}
		case <-timeout:
			t.Fatalf("timed out waiting for entry %q", want)
		}
	}
}

type writeFunc func(ctx context.Context, entries []loki.Entry, ctrls ...*componenttest.Controller) error

// runWriteTest runs fn as a subtest once for the receiver channel and once for Consume.
// The write func fans out entries to every given controller.
func runWriteTest(t *testing.T, fn func(t *testing.T, write writeFunc)) {
	t.Run("Receiver", func(t *testing.T) { fn(t, writeReceiver) })
	t.Run("Consume", func(t *testing.T) { fn(t, writeConsume) })
}

func writeReceiver(ctx context.Context, entries []loki.Entry, ctrls ...*componenttest.Controller) error {
	receivers := make([]loki.LogsReceiver, 0, len(ctrls))
	for _, ctrl := range ctrls {
		receivers = append(receivers, ctrl.Exports().(Exports).Receiver)
	}

	fanout := loki.NewFanout(receivers)
	for _, e := range entries {
		if err := fanout.Send(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func writeConsume(ctx context.Context, entries []loki.Entry, ctrls ...*componenttest.Controller) error {
	consumers := make([]loki.Consumer, 0, len(ctrls))
	for _, ctrl := range ctrls {
		comp, err := ctrl.GetComponent()
		if err != nil {
			return err
		}
		consumers = append(consumers, comp.(*Component))
	}

	batch := loki.NewBatch()
	for _, e := range entries {
		batch.AddEntry(e.Labels, 0, e.Entry)
	}
	return loki.NewFanoutConsumer(consumers).Consume(ctx, batch)
}
