package syslog_test

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/syslog"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Test performs a basic integration test which runs the otelcol.exporter.syslog
// component and ensures that it can pass data to a syslog receiver.
func Test(t *testing.T) {
	ch := make(chan string, 1)
	h, p := makeSyslogServer(t, ch)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.syslog")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		timeout = "250ms"
		endpoint = "%s"
		port = %s

		tls {
			insecure             = true
			insecure_skip_verify = true
		}

		debug_metrics {
			disable_high_cardinality_metrics = true
		}
	`, h, p)
	var args syslog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	require.Equal(t, args.DebugMetricsConfig().DisableHighCardinalityMetrics, true)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	// Truncating to second to make the syslog representation consistent, as the OTLP
	// representation doesn't show the sub-second precision consistently.
	timestamp := time.Now().Truncate(time.Second)

	// Send logs in the background to our exporter.
	go func() {
		exports := ctrl.Exports().(otelcol.ConsumerExports)

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			err := exports.Input.ConsumeLogs(ctx, createTestLogs(timestamp))
			if err != nil {
				level.Error(l).Log("msg", "failed to send logs", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our exporter to finish and pass data to our HTTP server.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for logs")
	case log := <-ch:
		// Order of the structured data is not guaranteed, so we need to check for both possible orders.
		expected := []string{
			fmt.Sprintf("<165>1 %s test-host Application 12345 - [Auth Realm=\"ADMIN\" User=\"root\"] This is a test log\n", timestamp.UTC().Format("2006-01-02T15:04:05Z")),
			fmt.Sprintf("<165>1 %s test-host Application 12345 - [Auth User=\"root\" Realm=\"ADMIN\"] This is a test log\n", timestamp.UTC().Format("2006-01-02T15:04:05Z")),
		}
		require.Contains(t, expected, log)
	}
}

func makeSyslogServer(t *testing.T, ch chan string) (string, string) {
	t.Helper()

	// Create a TCP listener on port 514
	conn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		c, err := conn.Accept()
		require.NoError(t, err)

		msg, err := io.ReadAll(c)
		require.NoError(t, err)

		err = c.Close()
		require.NoError(t, err)

		ch <- string(msg)
	}()

	t.Cleanup(func() { conn.Close() })

	h, p, err := net.SplitHostPort(conn.Addr().String())
	require.NoError(t, err)

	return h, p
}

func createTestLogs(t time.Time) plog.Logs {
	logs := plog.NewLogs()
	l := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	l.SetTimestamp(pcommon.NewTimestampFromTime(t))
	l.Attributes().PutStr("appname", "Application")
	l.Attributes().PutStr("hostname", "test-host")
	l.Attributes().PutStr("message", "This is a test log")
	l.Attributes().PutStr("proc_id", "12345")
	struc := map[string]any{
		"Auth": map[string]any{"Realm": "ADMIN", "User": "root"},
	}
	l.Attributes().PutEmptyMap("structured_data").FromRaw(struc)
	return logs
}
