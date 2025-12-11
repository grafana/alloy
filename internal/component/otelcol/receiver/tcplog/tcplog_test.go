package tcplog_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/tcplog"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Test performs a basic integration test which runs the otelcol.receiver.tcplog
// component and ensures that it can receive and forward data.
func Test(t *testing.T) {
	addr := componenttest.GetFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.tcplog")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		listen_address = "%s"

		output {
			// no-op: will be overridden by test code.
		}
	`, addr)

	var args tcplog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	// Override our settings so logs get forwarded to logsCh.
	logCh := make(chan plog.Logs)
	args.Output = makeLogsOutput(logCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(3*time.Second))

	time.Sleep(1 * time.Second)

	// Send logs in the background to our receiver.
	go func() {
		request := func() error {
			conn, err := net.Dial("tcp", addr)
			require.NoError(t, err)
			defer conn.Close()

			_, err = fmt.Fprintln(conn, "This is a test log message")
			return err
		}

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			if err := request(); err != nil {
				level.Error(l).Log("msg", "failed to send logs", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our client to get a log.
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
		listen_address = "localhost:1514"
		max_log_size = "2MiB"
		encoding = "utf-8"
		one_log_per_packet = true
		add_attributes = true

		tls {
			include_system_ca_certs_pool = true
			reload_interval = "1m"
		}

		multiline {
			line_start_pattern = "{"
			line_end_pattern = "}"
			omit_pattern = true
		}

		retry_on_failure {
			enabled = true
			initial_interval = "10s"
			max_interval = "1m"
			max_elapsed_time = "10m"
		}

		output {
		}
	`
	var args tcplog.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
}
