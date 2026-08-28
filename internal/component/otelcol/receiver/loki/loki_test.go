package loki

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"

	lokiapi "github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/loki/pkg/push"
)

func Test(t *testing.T) {
	ctx := componenttest.TestContext(t)
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
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))

	exports := ctrl.Exports().(Exports)

	// Use the exported receiver to send log entries in the background.
	go func() {
		entry := lokiapi.Entry{
			Labels: map[model.LabelName]model.LabelValue{
				"filename": "/var/log/app/errors.log",
				"env":      "dev",
			},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "It's super effective!",
			},
		}
		exports.Receiver.Chan() <- entry
	}()

	wantAttributes := map[string]any{
		"env":                   "dev",
		"filename":              "/var/log/app/errors.log",
		"log.file.name":         "errors.log",
		"log.file.path":         "/var/log/app/errors.log",
		"loki.attribute.labels": "filename,env",
	}

	// Wait for our client to get the log.
	var otelLogs plog.Logs
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for log entry")
	case otelLogs = <-logCh:
		require.Equal(t, 1, otelLogs.LogRecordCount())
		require.Equal(t, "It's super effective!", otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
		require.Equal(t, wantAttributes["env"], otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()["env"])
		require.Equal(t, wantAttributes["filename"], otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()["filename"])
		require.Equal(t, wantAttributes["log.file.name"], otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()["log.file.name"])
		require.Equal(t, wantAttributes["log.file.path"], otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()["log.file.path"])
		require.Contains(t, otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()["loki.attribute.labels"], "env")
		require.Contains(t, otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()["loki.attribute.labels"], "filename")
	}
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
