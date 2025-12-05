package filelog_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/filelog"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Test performs a basic integration test which runs the otelcol.receiver.filelog
// component and ensures that it can receive and forward data.
func Test(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	f, err := os.CreateTemp(t.TempDir(), "example")
	require.NoError(t, err)
	defer f.Close()

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.filelog")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		include = [%q]

		output {
			// no-op: will be overridden by test code.
		}
	`, f.Name())

	require.NoError(t, err)

	var args filelog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	// Override our settings so logs get forwarded to logsCh.
	logCh := make(chan plog.Logs)
	args.Output = makeLogsOutput(logCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(3*time.Second))

	// TODO(@dehaansa) - discover a better way to wait for otelcol component readiness
	time.Sleep(1 * time.Second)

	// Add a log message to the file
	fmt.Fprintf(f, "%s INFO test\n", time.Now().Format(time.RFC3339))

	// Wait for our client to get a span.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for logs")
	case log := <-logCh:
		require.Equal(t, 1, log.LogRecordCount())
	}
}

// makeLogsOutput returns ConsumerArguments which will forward logs to the
// provided channel.
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

func TestUnmarshal(t *testing.T) {
	alloyCfg := `
	include            = ["/var/log/*.log"]
	exclude            = ["/var/log/excluded.log"]
	exclude_older_than = "24h0m0s"
	ordering_criteria  {
		regex   = "^(?P<timestamp>\\d{8})_(?P<severity>\\d+)_"
		top_n   = 12
		group_by = "severity"

		sort_by  {
			sort_type = "timestamp"
			regex_key = "timestamp"
			ascending = true
			layout    = "%Y%m%d"
			location  = "UTC"
		}

		sort_by  {
			sort_type = "numeric"
			regex_key = "severity"
			ascending = true
		}
	}
	poll_interval              = "10s"
	max_concurrent_files       = 10
	max_batches                = 100
	start_at                   = "beginning"
	fingerprint_size           = "10KiB"
	max_log_size               = "10MiB"
	encoding                   = "utf-16"
	force_flush_period         = "5s"
	delete_after_read          = true
	include_file_record_number = true
	compression                = "gzip"
	acquire_fs_lock            = true

	header {
		pattern = "^HEADER .*$"
		metadata_operators = [{
			type = "regex_parser",
			regex = "^HEADER env='(?P<env>[^ ]+)'",
		}]
	}

	multiline {
		line_start_pattern = "\\d{4}-\\d{2}-\\d{2}"
		omit_pattern       = true
	}
	preserve_leading_whitespaces  = true
	preserve_trailing_whitespaces = true
	include_file_name             = true
	include_file_path             = true
	include_file_name_resolved    = true
	include_file_path_resolved    = true
	include_file_owner_name       = true
	include_file_owner_group_name = true
	attributes                    = {}
	resource                      = {}
	operators 					  = [{
      type = "regex_parser",
      regex = "^(?P<timestamp>[^ ]+)",
      timestamp = {
        parse_from = "attributes.timestamp",
        layout = "%Y-%m-%dT%H:%M:%S.%fZ",
        location = "UTC",
      },
    }]

	output {}
	`
	var args filelog.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	err = args.Validate()
	require.NoError(t, err)
}

func TestValidate(t *testing.T) {
	alloyCfg := `
	include            = ["/var/log/*.log"]
	exclude_older_than = "-5m"
	ordering_criteria  {
		regex   = "^(?P<timestamp>\\d{8})_(?P<severity>\\d+)_"
		top_n   = -3
		group_by = "severity"

		sort_by  {
			sort_type = "std_dev"
			regex_key = "severity"
			ascending = true
		}
	}
	poll_interval              = "-10s"
	max_concurrent_files       = 0
	max_batches                = -3
	start_at                   = "middle"
	fingerprint_size           = "-4KiB"
	max_log_size               = "-3MiB"
	encoding                   = "webdings"
	force_flush_period         = "-5s"
	compression                = "tar"

	header {
		pattern = "^HEADER .*$"
		metadata_operators = []
	}

	operators 					  = [{
      type = "regex_parser",
      regex = "^(?P<timestamp>[^ ]+)",
      timestamp = {
        parse_from = "timestamp",
        layout = "%Y-%m-%dT%H:%M:%S.%fZ",
        location = "UTC",
      },
    }]

	output {}
	`
	var args filelog.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.Error(t, err)
	require.Contains(t, err.Error(), "'parse_from' unrecognized prefix")
	require.Contains(t, err.Error(), "'max_concurrent_files' must be positive")
	require.Contains(t, err.Error(), "'max_batches' must not be negative")
	require.Contains(t, err.Error(), "invalid 'encoding': unsupported encoding 'webdings'")
	require.Contains(t, err.Error(), "'top_n' must not be negative")
	require.Contains(t, err.Error(), "invalid 'sort_type': std_dev")
	require.Contains(t, err.Error(), "invalid 'compression' type: tar")
	require.Contains(t, err.Error(), "'poll_interval' must not be negative")
	require.Contains(t, err.Error(), "'fingerprint_size' must not be negative")
	require.Contains(t, err.Error(), "'max_log_size' must not be negative")
	require.Contains(t, err.Error(), "'force_flush_period' must not be negative")
	require.Contains(t, err.Error(), "'header' requires at least one 'metadata_operator'")
	require.Contains(t, err.Error(), "'exclude_older_than' must not be negative")
}
