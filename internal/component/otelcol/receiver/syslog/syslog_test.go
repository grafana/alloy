package syslog_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/syslog"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Test performs a basic integration test which runs the otelcol.receiver.syslog
// component and ensures that it can receive and forward data.
func Test(t *testing.T) {
	tcp := componenttest.GetFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.syslog")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		protocol = "rfc5424"
		tcp {
			listen_address = "%s"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`, tcp)

	require.NoError(t, err)

	var args syslog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	require.Equal(t, "send", args.OnError)

	// Override our settings so logs get forwarded to logsCh.
	logCh := make(chan plog.Logs)
	args.Output = makeLogsOutput(logCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(3*time.Second))

	// TODO(@dehaansa) - test if this is removeable after https://github.com/grafana/alloy/pull/2262
	time.Sleep(1 * time.Second)

	// Send traces in the background to our receiver.
	go func() {
		request := func() error {
			conn, err := net.Dial("tcp", tcp)
			require.NoError(t, err)
			defer conn.Close()

			_, err = fmt.Fprint(conn, "<165>1 2018-10-11T22:14:15.003Z host5 e - id1 [custom@32473 exkey=\"1\"] An application event log entry...\n")
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
		protocol = "rfc5424"
		location = "UTC"
		enable_octet_counting = true
		max_octets = 16000
		allow_skip_pri_header = true
		non_transparent_framing_trailer = "NUL"
		on_error = "drop_quiet"
		tcp {
			listen_address = "localhost:1514"
			max_log_size = "2MiB"
			one_log_per_packet = true
			add_attributes = true
			encoding = "utf-16be"
			preserve_leading_whitespaces = true
			preserve_trailing_whitespaces = true
			tls {
				include_system_ca_certs_pool = true
				reload_interval = "1m"
			}
		}

		udp {
			listen_address = "localhost:1515"
			one_log_per_packet = false
			add_attributes = false
			encoding = "utf-16le"
			preserve_leading_whitespaces = false
			preserve_trailing_whitespaces = false
			async {
				readers = 2
				processors = 4
				max_queue_length = 1000
			}
			multiline {
				line_end_pattern = "logend"
				omit_pattern = true
			}

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
	var args syslog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(alloyCfg), &args))
	_, err := args.Convert()
	require.NoError(t, err)

	alloyTCP := `
		tcp {
			listen_address = "localhost:1514"
		}
		output {}
	`
	require.NoError(t, syntax.Unmarshal([]byte(alloyTCP), &args))
	_, err = args.Convert()
	require.NoError(t, err)

	alloyUDP := `
		udp {
			listen_address = "localhost:1514"
		}
		output {}
	`
	require.NoError(t, syntax.Unmarshal([]byte(alloyUDP), &args))
	_, err = args.Convert()
	require.NoError(t, err)
}

func TestValidateOnError(t *testing.T) {
	alloyCfg := `
		on_error = "invalid"
		output {
		}
	`
	var args syslog.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.ErrorContains(t, err, "invalid on_error: invalid")
}
