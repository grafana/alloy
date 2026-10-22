package write

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/lokitest"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
	"github.com/grafana/alloy/internal/featuregate"
	lokiutil "github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
	"github.com/grafana/alloy/syntax"
)

func TestWriteToSingleEndpoint(t *testing.T) {
	t.Run("wal disabled", func(t *testing.T) {
		lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
			testSingleEndpoint(t, env, func(args *Arguments) {})
		})
	})

	t.Run("wal enabled", func(t *testing.T) {
		lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
			testSingleEndpoint(t, env, func(args *Arguments) {
				args.WAL.Enabled = true
			})
		})
	})
}

func testSingleEndpoint(t *testing.T, env lokitest.Env, alterConfig func(arguments *Arguments)) {
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

	ctrl, _ := env.Run(t, "loki.write", args)

	// Send two log entries to the component's receiver
	logEntry := loki.Entry{
		Labels:    model.LabelSet{"foo": "bar"},
		Timestamp: time.Now(),
		Line:      "very important log",
	}

	env.Send(t, ctrl.Exports().(Exports).Receiver, logEntry, logEntry)

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
	t.Run("wal disabled", func(t *testing.T) {
		lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
			testMultipleEndpoint(t, env, func(arguments *Arguments) {})
		})
	})

	t.Run("wal enabled", func(t *testing.T) {
		lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
			testMultipleEndpoint(t, env, func(arguments *Arguments) {
				arguments.WAL.Enabled = true
			})
		})
	})
}

func testMultipleEndpoint(t *testing.T, env lokitest.Env, alterArgs func(arguments *Arguments)) {
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

	ctrl1, _ := env.Run(t, "loki.write", args1)
	ctrl2, _ := env.Run(t, "loki.write", args2)

	wantLabelSet := model.LabelSet{"somelbl": "somevalue"}
	entry := loki.NewEntry(wantLabelSet, push.Entry{
		Timestamp: time.Now(),
		Line:      "writing some text",
	})
	env.Send(t, ctrl1.Exports().(Exports).Receiver, entry)
	env.Send(t, ctrl2.Exports().(Exports).Receiver, entry)

	// Each endpoint receives the entry with its component's external labels merged in.
	for range 2 {
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
	getServiceData := func(name string) (any, error) {
		if name == lokipipelinev2.ServiceName {
			return lokipipelinev2.New(lokipipelinev2.Options{}).Data(), nil
		}
		return nil, fmt.Errorf("service not found %s", name)
	}

	t.Run("should be able to create component with default queue_config", func(t *testing.T) {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			endpoint {
				url = "test.com"
			}
		`), &args)
		require.NoError(t, err)

		_, err = New(component.Options{
			Logger:         logging.NewSlogNop(),
			MinStability:   featuregate.StabilityGenerallyAvailable,
			OnStateChange:  func(e component.Exports) {},
			GetServiceData: getServiceData,
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
			Logger:         logging.NewSlogNop(),
			MinStability:   featuregate.StabilityGenerallyAvailable,
			OnStateChange:  func(e component.Exports) {},
			GetServiceData: getServiceData,
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
			Logger:         logging.NewSlogNop(),
			MinStability:   featuregate.StabilityExperimental,
			OnStateChange:  func(e component.Exports) {},
			GetServiceData: getServiceData,
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
			Logger:         logging.NewSlogNop(),
			MinStability:   featuregate.StabilityGenerallyAvailable,
			OnStateChange:  func(e component.Exports) {},
			GetServiceData: getServiceData,
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
			Logger:         logging.NewSlogNop(),
			MinStability:   featuregate.StabilityGenerallyAvailable,
			OnStateChange:  func(e component.Exports) {},
			GetServiceData: getServiceData,
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

		c, err := New(component.Options{
			Logger:         logging.NewSlogNop(),
			MinStability:   featuregate.StabilityExperimental,
			DataPath:       t.TempDir(),
			OnStateChange:  func(e component.Exports) {},
			GetServiceData: getServiceData,
		}, args)
		require.NoError(t, err)

		// Run with a canceled context to close the WAL. Windows cannot remove
		// the TempDir while a WAL segment is open.
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, c.Run(ctx))
	})
}

func TestUpdateWhileSendingIsBlocked(t *testing.T) {
	for _, walEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("wal_enabled=%t", walEnabled), func(t *testing.T) {
			lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
				ctrl, _ := blockedComponent(t, env, walEnabled)

				updated := make(chan error, 1)
				go func() { updated <- ctrl.Update(blockedEndpointArgs(t, "http://localhost:1", walEnabled)) }()

				select {
				case err := <-updated:
					require.NoError(t, err)
				case <-time.After(30 * time.Second):
					t.Fatal("Update did not return while sending was blocked")
				}
			})
		})
	}
}

func TestStopWhileSendingIsBlocked(t *testing.T) {
	for _, walEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("wal_enabled=%t", walEnabled), func(t *testing.T) {
			lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
				_, stop := blockedComponent(t, env, walEnabled)
				stop()
			})
		})
	}
}

func blockedComponent(t *testing.T, env lokitest.Env, walEnabled bool) (*componenttest.Controller, func()) {
	t.Helper()

	blocked := atomic.NewBool(false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blocked.Store(true)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	ctrl, stop := env.Run(t, "loki.write", blockedEndpointArgs(t, srv.URL, walEnabled), func(o component.Options) component.Options {
		o.MinStability = featuregate.StabilityExperimental
		return o
	})

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	recv := ctrl.Exports().(Exports).Receiver
	send := func(e loki.Entry) {
		if c, ok := recv.(loki.Consumer); ok {
			batch := loki.NewBatch()
			batch.AddEntry(e.Labels, e.Created(), e.Entry)
			_ = c.Consume(ctx, batch)
			return
		}

		select {
		case recv.Chan() <- e:
		case <-ctx.Done():
		}
	}

	go func() {
		entry := loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "very important log"})
		// Sends block once the queue is full. Once the component is stopped a
		// pipeline v2 send returns right away, so ctx is checked to stop.
		for ctx.Err() == nil {
			send(entry)
		}
	}()

	require.Eventually(t, blocked.Load, 10*time.Second, 10*time.Millisecond)
	return ctrl, stop
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
	for _, walEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("wal_enabled=%t", walEnabled), func(t *testing.T) {
			lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
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

				ctrl, _ := env.Run(t, "loki.write", firstArgs)
				recv := ctrl.Exports().(Exports).Receiver

				env.Send(t, recv, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "first"}))
				waitForLine(t, firstReceived, "first")

				require.Error(t, ctrl.Update(failingArgs))

				// The failed update left the previous consumer in place.
				env.Send(t, recv, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "second"}))
				waitForLine(t, firstReceived, "second")

				require.NoError(t, ctrl.Update(secondArgs))

				env.Send(t, recv, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "third"}))
				waitForLine(t, secondReceived, "third")
			})
		})
	}
}

func TestUpdateTogglesWAL(t *testing.T) {
	lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
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

		ctrl, _ := env.Run(t, "loki.write", walArgs)
		recv := ctrl.Exports().(Exports).Receiver

		env.Send(t, recv, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "with wal"}))
		waitForLine(t, received, "with wal")

		require.NoError(t, ctrl.Update(noWalArgs))

		env.Send(t, recv, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "without wal"}))
		waitForLine(t, received, "without wal")

		require.NoError(t, ctrl.Update(walArgs))

		env.Send(t, recv, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "with wal again"}))
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

func TestRetriesFailedWALWrite(t *testing.T) {
	runWithPipelines(t, func(t *testing.T, pipelineV2 bool) {
		synctest.Test(t, func(t *testing.T) {
			c, wl, stop := newFakeWALComponent(t, pipelineV2, 2)

			require.NoError(t, sendEntry(t.Context(), c, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "after retry"})))

			// Pipeline V1 retries after the send returns.
			time.Sleep(time.Second)
			stop()

			require.Equal(t, int64(3), wl.calls.Load())
			require.Len(t, wl.entries, 1)
		})
	})
}

func TestStoppedWhileWaitingForWALRetry(t *testing.T) {
	runWithPipelines(t, func(t *testing.T, pipelineV2 bool) {
		synctest.Test(t, func(t *testing.T) {
			c, wl, stop := newFakeWALComponent(t, pipelineV2, math.MaxInt64)

			sent := make(chan struct{})
			go func() {
				defer close(sent)
				_ = sendEntry(t.Context(), c, loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "never written"}))
			}()

			// Lets the backoff grow to its max delay.
			time.Sleep(time.Minute)

			start := time.Now()
			stop()
			<-sent

			require.Less(t, time.Since(start), time.Second)
			require.Len(t, wl.entries, 0)
		})
	})
}

// runWithPipelines runs fn as a subtest for each Loki pipeline.
func runWithPipelines(t *testing.T, fn func(t *testing.T, pipelineV2 bool)) {
	t.Run("pipeline v1", func(t *testing.T) { fn(t, false) })
	t.Run("pipeline v2", func(t *testing.T) { fn(t, true) })
}

// sendEntry sends e to the receiver of c.
func sendEntry(ctx context.Context, c *Component, e loki.Entry) error {
	if consumer, ok := c.receiver.(loki.Consumer); ok {
		batch := loki.NewBatch()
		batch.AddEntry(e.Labels, e.Created(), e.Entry)
		return consumer.Consume(ctx, batch)
	}

	select {
	case c.receiver.Chan() <- e:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newFakeWALComponent(t *testing.T, pipelineV2 bool, failures int64) (*Component, *fakeWAL, func()) {
	t.Helper()

	wl := &fakeWAL{dir: t.TempDir(), failures: failures}

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		endpoint {
			url = "http://localhost:1"
		}

		wal {
			enabled       = true
			drain_timeout = "1ms"
		}
	`), &args))

	c := &Component{
		opts: component.Options{
			Logger:     logging.NewSlogNop(),
			Registerer: prometheus.NewRegistry(),
		},
		pipelineV2: pipelineV2,
		wal:        wl,
	}

	c.stopCtx, c.stop = context.WithCancel(context.Background())

	if pipelineV2 {
		c.receiver = loki.NewLogsReceiverV2(c, "loki.write.test")
	} else {
		c.receiver = loki.NewLogsReceiver()
	}

	consumer, err := c.newWALConsumer(args, args.convertEndpointConfigs())
	require.NoError(t, err)
	c.consumer = consumer
	consumer.Start()

	runCtx, cancelRun := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Run(runCtx)
	}()

	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancelRun()
			<-done
		})
	}
	t.Cleanup(stop)

	return c, wl, stop
}

// fakeWAL fails the first failures writes and keeps the entries of the rest.
type fakeWAL struct {
	dir      string
	failures int64
	calls    atomic.Int64
	entries  []push.Entry
}

func (w *fakeWAL) Log(r *wal.Record) error {
	if w.calls.Inc() <= w.failures {
		return errors.New("no space left on device")
	}
	// The record is reused after Log returns, so the entries are copied out.
	for _, e := range r.RefEntries {
		w.entries = append(w.entries, e.Entries...)
	}
	return nil
}

func (w *fakeWAL) Sync() error               { return nil }
func (w *fakeWAL) Dir() string               { return w.dir }
func (w *fakeWAL) Close()                    {}
func (w *fakeWAL) NextSegment() (int, error) { return 0, nil }

func TestUpdateKeepsConsumerWhenSettingsAreUnchanged(t *testing.T) {
	for _, walEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("wal_enabled=%t", walEnabled), func(t *testing.T) {
			newArgs := func(url string, externalLabels string) Arguments {
				var args Arguments
				require.NoError(t, syntax.Unmarshal([]byte(fmt.Sprintf(`
					endpoint {
						url        = "%s"
						batch_wait = "10ms"
						tenant_id  = "tenant"
						headers    = {"X-Custom" = "value"}
						basic_auth {
							username = "user"
							password = "secret"
						}
						tls_config {
							insecure_skip_verify = true
						}
					}
					external_labels = %s
				`, url, externalLabels)), &args))
				args.WAL.Enabled = walEnabled
				return args
			}

			c, err := New(component.Options{
				ID:            "loki.write.test",
				Logger:        logging.NewSlogNop(),
				Registerer:    prometheus.NewRegistry(),
				DataPath:      t.TempDir(),
				MinStability:  featuregate.StabilityExperimental,
				OnStateChange: func(component.Exports) {},
			}, newArgs("http://localhost:1", `{}`))
			require.NoError(t, err)
			t.Cleanup(func() {
				c.consumer.Stop()
				if c.wal != nil {
					c.wal.Close()
				}
			})
			first := c.consumer

			require.NoError(t, c.Update(newArgs("http://localhost:1", `{}`)))
			require.True(t, first == c.consumer, "consumer was restarted although nothing changed")

			require.NoError(t, c.Update(newArgs("http://localhost:1", `{cluster = "a"}`)))
			require.True(t, first == c.consumer, "consumer was restarted although only external_labels changed")
			require.Equal(t, model.LabelSet{"cluster": "a"}, c.externalLabels)

			require.NoError(t, c.Update(newArgs("http://localhost:2", `{cluster = "a"}`)))
			require.False(t, first == c.consumer, "consumer was not restarted after the endpoint changed")
		})
	}
}

func BenchmarkLokiWrite(b *testing.B) {
	setup := func(b *testing.B, env lokitest.Env, delivered func(n int)) loki.LogsReceiver {
		ch := make(chan lokiutil.RemoteWriteRequest)
		go func() {
			for {
				select {
				case req := <-ch:
					for _, s := range req.Request.Streams {
						delivered(len(s.Entries))
					}
				case <-b.Context().Done():
					return
				}
			}
		}()

		srv := lokiutil.NewRemoteWriteServer(ch, http.StatusOK)
		b.Cleanup(srv.Close)

		cfg := fmt.Sprintf(`
			endpoint {
				url        = "%s"
				batch_wait = "10ms"
				tenant_id  = "tenant-1"
			}
		`, srv.URL)
		var args Arguments
		require.NoError(b, syntax.Unmarshal([]byte(cfg), &args))

		ctrl, _ := env.Run(b, "loki.write", args)
		return ctrl.Exports().(Exports).Receiver
	}

	newBatches := func(workers int) []loki.Batch {
		const (
			streamsPerBatch  = 10
			entriesPerStream = 50
		)

		now := time.Now()
		batches := make([]loki.Batch, workers)
		for w := range batches {
			for i := range streamsPerBatch {
				entries := make([]push.Entry, entriesPerStream)
				for j := range entries {
					entries[j] = push.Entry{Timestamp: now, Line: "very important log"}
				}
				batches[w].Add(loki.NewStream(
					model.LabelSet{"foo": model.LabelValue(fmt.Sprintf("bar-%d-%d", w, i))},
					entries...,
				))
			}
		}
		return batches
	}

	b.Run("single sender", func(b *testing.B) {
		lokitest.BenchmarkSink(b, newBatches(1), setup)
	})

	b.Run("multiple senders", func(b *testing.B) {
		lokitest.BenchmarkSink(b, newBatches(8), setup)
	})
}
