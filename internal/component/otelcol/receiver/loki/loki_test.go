package loki

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
)

func TestComponent(t *testing.T) {
	t.Run("LogReceiver", func(t *testing.T) {
		l := util.TestLogger(t)
		ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.loki")
		require.NoError(t, err)

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

		go func() {
			require.NoError(t, ctrl.Run(t.Context(), args))
		}()

		require.NoError(t, ctrl.WaitRunning(time.Second))
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

	t.Run("Consumer", func(t *testing.T) {
		var (
			now  = time.Now()
			recv = make(chan plog.Logs)
		)

		c, err := New(component.Options{
			Logger:        slog.New(slog.DiscardHandler),
			OnStateChange: func(e component.Exports) {},
			GetServiceData: func(string) (any, error) {
				return livedebugging.NewLiveDebugging(), nil
			},
		}, Arguments{
			Output: makeLogsOutput(recv),
		})
		require.NoError(t, err)

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
	})

	t.Run("Consumer stopped", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		c, err := New(component.Options{
			Logger:        slog.New(slog.DiscardHandler),
			OnStateChange: func(e component.Exports) {},
			GetServiceData: func(string) (any, error) {
				return livedebugging.NewLiveDebugging(), nil
			},
		}, Arguments{
			Output: makeLogsOutput(make(chan plog.Logs, 1)),
		})
		require.NoError(t, err)

		runErr := make(chan error, 1)
		go func() { runErr <- c.Run(ctx) }()

		cancel()
		require.NoError(t, <-runErr)

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(
			model.LabelSet{"stream": "1"},
			push.Entry{Line: "1", Timestamp: time.Now()},
		))

		require.ErrorIs(t, c.Consume(t.Context(), batch), loki.ErrConsumerStopped)
	})

	t.Run("Consumer empty batch", func(t *testing.T) {
		recv := make(chan plog.Logs, 1)
		c, err := New(component.Options{
			Logger:        slog.New(slog.DiscardHandler),
			OnStateChange: func(e component.Exports) {},
			GetServiceData: func(string) (any, error) {
				return livedebugging.NewLiveDebugging(), nil
			},
		}, Arguments{
			Output: makeLogsOutput(recv),
		})
		require.NoError(t, err)
		require.NoError(t, c.Consume(t.Context(), loki.NewBatch()))

		select {
		case <-recv:
			require.FailNow(t, "empty batch should not forward logs")
		default:
		}
	})
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
