package loki

import (
	"context"
	"sort"
	"strings"
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
		"loki.attribute.labels": "env,filename",
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
		// The hint attribute is now sorted, so check for the sorted value.
		hintVal := otelLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()["loki.attribute.labels"].(string)
		require.Contains(t, hintVal, "env")
		require.Contains(t, hintVal, "filename")
	}
}

func TestConvertLokiEntryToPlog_IncludeLabels(t *testing.T) {
	entry := lokiapi.Entry{
		Labels: map[model.LabelName]model.LabelValue{
			"job":      "myapp",
			"instance": "host123:8080",
			"level":    "error",
			"stream":   "stdout",
		},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "test log line",
		},
	}

	cfg := &LabelsConfig{
		Include: []string{"job", "instance"},
	}

	logs := convertLokiEntryToPlog(entry, cfg)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()

	// Only "job" and "instance" should be present as label-derived attributes.
	require.Equal(t, "myapp", attrs["job"])
	require.Equal(t, "host123:8080", attrs["instance"])

	// "level" and "stream" should NOT be present.
	_, hasLevel := attrs["level"]
	require.False(t, hasLevel, "level should be excluded")
	_, hasStream := attrs["stream"]
	require.False(t, hasStream, "stream should be excluded")

	// The hint attribute should only contain the included labels.
	hintVal := attrs["loki.attribute.labels"].(string)
	parts := strings.Split(hintVal, ",")
	sort.Strings(parts)
	require.Equal(t, []string{"instance", "job"}, parts)
}

func TestConvertLokiEntryToPlog_ExcludeLabels(t *testing.T) {
	entry := lokiapi.Entry{
		Labels: map[model.LabelName]model.LabelValue{
			"job":      "myapp",
			"instance": "host123:8080",
			"level":    "error",
			"stream":   "stdout",
		},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "test log line",
		},
	}

	cfg := &LabelsConfig{
		Exclude: []string{"level", "stream"},
	}

	logs := convertLokiEntryToPlog(entry, cfg)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()

	// "job" and "instance" should be present.
	require.Equal(t, "myapp", attrs["job"])
	require.Equal(t, "host123:8080", attrs["instance"])

	// "level" and "stream" should NOT be present.
	_, hasLevel := attrs["level"]
	require.False(t, hasLevel, "level should be excluded")
	_, hasStream := attrs["stream"]
	require.False(t, hasStream, "stream should be excluded")

	// The hint attribute should only contain the non-excluded labels.
	hintVal := attrs["loki.attribute.labels"].(string)
	parts := strings.Split(hintVal, ",")
	sort.Strings(parts)
	require.Equal(t, []string{"instance", "job"}, parts)
}

func TestConvertLokiEntryToPlog_RenameLabels(t *testing.T) {
	entry := lokiapi.Entry{
		Labels: map[model.LabelName]model.LabelValue{
			"job": "myapp",
			"env": "production",
		},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "test log line",
		},
	}

	cfg := &LabelsConfig{
		Rename: map[string]string{
			"job": "service.name",
		},
	}

	logs := convertLokiEntryToPlog(entry, cfg)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()

	// "job" should be renamed to "service.name".
	require.Equal(t, "myapp", attrs["service.name"])
	_, hasJob := attrs["job"]
	require.False(t, hasJob, "original 'job' key should be removed after rename")

	// "env" should remain unchanged.
	require.Equal(t, "production", attrs["env"])

	// The hint attribute should use the renamed key.
	hintVal := attrs["loki.attribute.labels"].(string)
	parts := strings.Split(hintVal, ",")
	sort.Strings(parts)
	require.Equal(t, []string{"env", "service.name"}, parts)
}

func TestConvertLokiEntryToPlog_IncludeAndRename(t *testing.T) {
	entry := lokiapi.Entry{
		Labels: map[model.LabelName]model.LabelValue{
			"job":      "myapp",
			"instance": "host123:8080",
			"level":    "error",
		},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "test log line",
		},
	}

	cfg := &LabelsConfig{
		Include: []string{"job"},
		Rename: map[string]string{
			"job": "service.name",
		},
	}

	logs := convertLokiEntryToPlog(entry, cfg)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()

	// Only "job" was included, and it's renamed to "service.name".
	require.Equal(t, "myapp", attrs["service.name"])
	_, hasJob := attrs["job"]
	require.False(t, hasJob, "original 'job' key should be removed")
	_, hasInstance := attrs["instance"]
	require.False(t, hasInstance, "instance should be excluded by include filter")
	_, hasLevel := attrs["level"]
	require.False(t, hasLevel, "level should be excluded by include filter")

	// Hint should only contain the renamed label.
	hintVal := attrs["loki.attribute.labels"].(string)
	require.Equal(t, "service.name", hintVal)
}

func TestConvertLokiEntryToPlog_NilConfig(t *testing.T) {
	// This verifies backward compatibility: nil config forwards all labels.
	entry := lokiapi.Entry{
		Labels: map[model.LabelName]model.LabelValue{
			"job":   "myapp",
			"level": "info",
		},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "test log line",
		},
	}

	logs := convertLokiEntryToPlog(entry, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()

	require.Equal(t, "myapp", attrs["job"])
	require.Equal(t, "info", attrs["level"])

	hintVal := attrs["loki.attribute.labels"].(string)
	parts := strings.Split(hintVal, ",")
	sort.Strings(parts)
	require.Equal(t, []string{"job", "level"}, parts)
}

func TestConvertLokiEntryToPlog_FilenameAlwaysProcessed(t *testing.T) {
	// Even when filename is excluded, log.file.path and log.file.name are still set.
	entry := lokiapi.Entry{
		Labels: map[model.LabelName]model.LabelValue{
			"filename": "/var/log/app.log",
			"job":      "myapp",
		},
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "test log line",
		},
	}

	cfg := &LabelsConfig{
		Exclude: []string{"filename"},
	}

	logs := convertLokiEntryToPlog(entry, cfg)
	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()

	// log.file.path and log.file.name are always added when filename label exists.
	require.Equal(t, "/var/log/app.log", attrs["log.file.path"])
	require.Equal(t, "app.log", attrs["log.file.name"])

	// But the "filename" label itself should be excluded from the forwarded attributes.
	_, hasFilename := attrs["filename"]
	require.False(t, hasFilename, "filename label should be excluded")

	// "job" should be present.
	require.Equal(t, "myapp", attrs["job"])
}

func TestValidate_IncludeAndExcludeMutuallyExclusive(t *testing.T) {
	args := Arguments{
		Labels: &LabelsConfig{
			Include: []string{"job"},
			Exclude: []string{"level"},
		},
	}
	err := args.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_IncludeOnly(t *testing.T) {
	args := Arguments{
		Labels: &LabelsConfig{
			Include: []string{"job"},
		},
	}
	require.NoError(t, args.Validate())
}

func TestValidate_ExcludeOnly(t *testing.T) {
	args := Arguments{
		Labels: &LabelsConfig{
			Exclude: []string{"level"},
		},
	}
	require.NoError(t, args.Validate())
}

func TestValidate_NoLabelsBlock(t *testing.T) {
	args := Arguments{}
	require.NoError(t, args.Validate())
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
