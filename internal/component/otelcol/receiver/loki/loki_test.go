package loki

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/lokitest"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
)

func TestLabelsAsResourceAttributes(t *testing.T) {
	lokitest.Test(t, func(t *testing.T, env lokitest.Env) {
		var args Arguments
		require.NoError(t, syntax.Unmarshal([]byte(`
			labels_as_resource_attributes = true
			output {}
		`), &args))

		recv := make(chan plog.Logs, 10)
		args.Output = makeLogsOutput(recv)

		ctrl, _ := env.Run(t, "otelcol.receiver.loki", args)
		require.NoError(t, ctrl.WaitExports(time.Second))
		receiver := ctrl.Exports().(Exports).Receiver

		now := time.Now()
		send(t, receiver,
			loki.NewStream(
				model.LabelSet{"env": "dev", "filename": "/var/log/app.log", "__tenant_id__": "tenant"},
				push.Entry{Line: "1", Timestamp: now, StructuredMetadata: push.LabelsAdapter{{Name: "trace_id", Value: "abc"}}},
				push.Entry{Line: "2", Timestamp: now},
			),
			loki.NewStream(
				model.LabelSet{"env": "prod"},
				push.Entry{Line: "3", Timestamp: now},
			),
		)

		devResource := map[string]any{"env": "dev", "filename": "/var/log/app.log"}
		want := []resourceLogRecord{
			{resource: devResource, body: "1", attributes: map[string]any{"trace_id": "abc"}},
			{resource: devResource, body: "2", attributes: map[string]any{}},
			{resource: map[string]any{"env": "prod"}, body: "3", attributes: map[string]any{}},
		}
		require.Equal(t, want, collectResourceLogRecords(t, recv, len(want)))
	})
}

// send delivers streams to receiver through Consume for pipeline v2, or entry by entry over Chan for v1.
func send(t *testing.T, receiver loki.LogsReceiver, streams ...loki.Stream) {
	t.Helper()

	if c, ok := receiver.(loki.Consumer); ok {
		batch := loki.NewBatch()
		for _, s := range streams {
			batch.Add(s)
		}
		require.NoError(t, c.Consume(t.Context(), batch))
		return
	}

	for _, s := range streams {
		for _, e := range s.Entries {
			select {
			case receiver.Chan() <- loki.NewEntry(s.Labels, e):
			case <-time.After(time.Second):
				require.FailNow(t, "timed out sending entry")
			}
		}
	}
}

type resourceLogRecord struct {
	resource   map[string]any
	body       string
	attributes map[string]any
}

// collectResourceLogRecords reads from recv until n log records arrived and flattens them with their resource.
func collectResourceLogRecords(t *testing.T, recv chan plog.Logs, n int) []resourceLogRecord {
	t.Helper()

	var got []resourceLogRecord
	for len(got) < n {
		select {
		case logs := <-recv:
			for _, rl := range logs.ResourceLogs().All() {
				for _, sl := range rl.ScopeLogs().All() {
					for _, lr := range sl.LogRecords().All() {
						got = append(got, resourceLogRecord{
							resource:   rl.Resource().Attributes().AsRaw(),
							body:       lr.Body().AsString(),
							attributes: lr.Attributes().AsRaw(),
						})
					}
				}
			}
		case <-time.After(2 * time.Second):
			require.FailNowf(t, "timeout", "got %d of %d log records", len(got), n)
		}
	}
	return got
}

func TestComponent(t *testing.T) {
	lokitest.TestV1(t, func(t *testing.T, env lokitest.Env) {
		t.Run("LogReceiver", func(t *testing.T) {
			cfg := `
			output {
				// no-op: will be overridden by test code.
			}
		`
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

			// Override our settings so logs get forwarded to logCh.
			logCh := make(chan plog.Logs)
			args.Output = makeLogsOutput(logCh)

			ctrl, _ := env.Run(t, "otelcol.receiver.loki", args)
			require.NoError(t, ctrl.WaitExports(time.Second))

			exports := ctrl.Exports().(Exports)

			now := time.Now()

			// Use the exported receiver to send log entries in the background.
			go func() {
				entry := loki.Entry{
					Labels: map[model.LabelName]model.LabelValue{
						"filename": "/var/log/app/errors.log",
						"env":      "dev",
					},
					Entry: push.Entry{
						Timestamp: now,
						Line:      "It's super effective!",
					},
				}
				exports.Receiver.Chan() <- entry
			}()

			// Wait for our client to get the log.
			select {
			case <-time.After(time.Second):
				require.FailNow(t, "failed waiting for log entry")
			case otelLogs := <-logCh:
				requireLogRecords(t, otelLogs, []logRecord{
					{
						body:      "It's super effective!",
						timestamp: now,
						attributes: map[string]any{
							"env":                   "dev",
							"filename":              "/var/log/app/errors.log",
							"log.file.name":         "errors.log",
							"log.file.path":         "/var/log/app/errors.log",
							"loki.attribute.labels": "env,filename",
						},
					},
				})
			}
		})
	})

	lokitest.TestV2(t, func(t *testing.T, env lokitest.Env) {
		t.Run("Consumer", func(t *testing.T) {
			testConsumer(t, env)
		})
		t.Run("Consumer stopped", func(t *testing.T) {
			testConsumerStopped(t, env)
		})
		t.Run("Consumer empty batch", func(t *testing.T) {
			testConsumerEmptyBatch(t, env)
		})
		t.Run("Update and stop while downstream is blocked", func(t *testing.T) {
			testBlockedDownstream(t, env)
		})
	})
}

// runV2 runs the component in env and returns its exported consumer.
func runV2(t *testing.T, env lokitest.Env, recv chan plog.Logs) (loki.Consumer, func()) {
	t.Helper()

	ctrl, stop := env.Run(t, "otelcol.receiver.loki", Arguments{Output: makeLogsOutput(recv)})
	require.NoError(t, ctrl.WaitExports(time.Second))

	c, ok := ctrl.Exports().(Exports).Receiver.(loki.Consumer)
	require.True(t, ok, "pipeline v2 receiver must implement loki.Consumer")
	return c, stop
}

func testConsumer(t *testing.T, env lokitest.Env) {
	var (
		now  = time.Now()
		recv = make(chan plog.Logs)
	)

	c, _ := runV2(t, env, recv)

	batch := loki.NewBatch()
	batch.Add(loki.NewStream(
		model.LabelSet{"stream": "1", "env": "dev", "filename": "/path1/file1"},
		push.Entry{Line: "1", Timestamp: now},
		push.Entry{Line: "2", Timestamp: now.Add(time.Second)},
		push.Entry{Line: "3", Timestamp: now.Add(2 * time.Second)},
	))
	batch.Add(loki.NewStream(
		model.LabelSet{"stream": "2", "env": "prod", "filename": "/path2/file2"},
		push.Entry{Line: "1", Timestamp: now},
		push.Entry{Line: "2", Timestamp: now.Add(time.Second)},
	))

	go func() {
		require.NoError(t, c.Consume(t.Context(), batch))
	}()

	select {
	case logs := <-recv:
		requireLogRecords(t, logs, []logRecord{
			{
				body:      "1",
				timestamp: now,
				attributes: map[string]any{
					"stream":                "1",
					"env":                   "dev",
					"filename":              "/path1/file1",
					"log.file.name":         "file1",
					"log.file.path":         "/path1/file1",
					"loki.attribute.labels": "env,filename,stream",
				},
			},
			{
				body:      "2",
				timestamp: now.Add(time.Second),
				attributes: map[string]any{
					"stream":                "1",
					"env":                   "dev",
					"filename":              "/path1/file1",
					"log.file.name":         "file1",
					"log.file.path":         "/path1/file1",
					"loki.attribute.labels": "env,filename,stream",
				},
			},
			{
				body:      "3",
				timestamp: now.Add(2 * time.Second),
				attributes: map[string]any{
					"stream":                "1",
					"env":                   "dev",
					"filename":              "/path1/file1",
					"log.file.name":         "file1",
					"log.file.path":         "/path1/file1",
					"loki.attribute.labels": "env,filename,stream",
				},
			},
			{
				body:      "1",
				timestamp: now,
				attributes: map[string]any{
					"stream":                "2",
					"env":                   "prod",
					"filename":              "/path2/file2",
					"log.file.name":         "file2",
					"log.file.path":         "/path2/file2",
					"loki.attribute.labels": "env,filename,stream",
				},
			},
			{
				body:      "2",
				timestamp: now.Add(time.Second),
				attributes: map[string]any{
					"stream":                "2",
					"env":                   "prod",
					"filename":              "/path2/file2",
					"log.file.name":         "file2",
					"log.file.path":         "/path2/file2",
					"loki.attribute.labels": "env,filename,stream",
				},
			},
		})

	case <-time.After(2 * time.Second):
		require.FailNow(t, "logs did not arrive in time")
	}
}

func testConsumerStopped(t *testing.T, env lokitest.Env) {
	c, stop := runV2(t, env, make(chan plog.Logs, 1))
	stop()

	batch := loki.NewBatch()
	batch.Add(loki.NewStream(
		model.LabelSet{"stream": "1"},
		push.Entry{Line: "1", Timestamp: time.Now()},
	))

	require.ErrorIs(t, c.Consume(t.Context(), batch), loki.ErrConsumerStopped)
}

func testConsumerEmptyBatch(t *testing.T, env lokitest.Env) {
	recv := make(chan plog.Logs, 1)
	c, _ := runV2(t, env, recv)
	require.NoError(t, c.Consume(t.Context(), loki.NewBatch()))

	select {
	case <-recv:
		require.FailNow(t, "empty batch should not forward logs")
	default:
	}
}

func testBlockedDownstream(t *testing.T, env lokitest.Env) {
	entered := make(chan struct{}, 1)
	blocking := &fakeconsumer.Consumer{
		ConsumeLogsFunc: func(ctx context.Context, _ plog.Logs) error {
			entered <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		},
	}
	args := Arguments{Output: &otelcol.ConsumerArguments{Logs: []otelcol.Consumer{blocking}}}

	ctrl, stop := env.Run(t, "otelcol.receiver.loki", args)
	require.NoError(t, ctrl.WaitExports(time.Second))
	c := ctrl.Exports().(Exports).Receiver.(loki.Consumer)

	consumeCtx, cancelConsume := context.WithCancel(t.Context())
	defer cancelConsume()

	batch := loki.NewBatch()
	batch.Add(loki.NewStream(model.LabelSet{"stream": "1"}, push.Entry{Line: "1", Timestamp: time.Now()}))
	consumed := make(chan error, 1)
	go func() { consumed <- c.Consume(consumeCtx, batch) }()
	<-entered

	require.NoError(t, requireReturns(t, "Update", func() error { return ctrl.Update(args) }))
	// stop fails the test itself if Run doesn't exit in time.
	stop()

	cancelConsume()
	require.ErrorIs(t, <-consumed, context.Canceled)
}

// requireReturns fails the test if fn does not return within 5 seconds.
func requireReturns(t *testing.T, name string, fn func() error) error {
	t.Helper()

	done := make(chan error, 1)
	go func() { done <- fn() }()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		require.FailNowf(t, "blocked", "%s did not return while downstream was blocked", name)
		return nil
	}
}

type logRecord struct {
	body       string
	timestamp  time.Time
	attributes map[string]any
}

func requireLogRecords(t *testing.T, logs plog.Logs, want []logRecord) {
	t.Helper()

	got := make([]logRecord, 0, logs.LogRecordCount())
	for _, rl := range logs.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			for _, lr := range sl.LogRecords().All() {
				got = append(got, logRecord{
					body:       lr.Body().AsString(),
					timestamp:  lr.Timestamp().AsTime(),
					attributes: lr.Attributes().AsRaw(),
				})
			}
		}
	}

	for i := range want {
		want[i].timestamp = pcommon.NewTimestampFromTime(want[i].timestamp).AsTime()
	}

	require.Equal(t, want, got)
}

// makeLogsOutput returns a ConsumerArguments which will forward logs to
// the provided channel.
func makeLogsOutput(ch chan plog.Logs) *otelcol.ConsumerArguments {
	logsConsumer := fakeconsumer.Consumer{
		ConsumeLogsFunc: func(ctx context.Context, l plog.Logs) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- l:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Logs: []otelcol.Consumer{&logsConsumer},
	}
}
