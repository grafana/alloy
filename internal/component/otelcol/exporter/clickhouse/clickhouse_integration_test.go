//go:build integration

package clickhouse_test

import (
	"context"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	chcomponent "github.com/grafana/alloy/internal/component/otelcol/exporter/clickhouse"
)

// TestIntegration_LogsRoundTrip starts a real ClickHouse container, builds the
// exporter the same way otelcol.exporter.clickhouse does (Arguments -> Convert
// -> upstream factory), pushes one log record through it, and confirms the row
// actually landed in ClickHouse. This is the check that would catch a
// regression in Convert()'s field mapping that a mocked/absent-ClickHouse unit
// test can't catch.
func TestIntegration_LogsRoundTrip(t *testing.T) {
	ctx := context.Background()

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "clickhouse/clickhouse-server:latest",
			ExposedPorts: []string{"9000/tcp"},
			// The official image requires CLICKHOUSE_PASSWORD to provision the
			// default user with a password; without it, native-protocol auth fails.
			Env:        map[string]string{"CLICKHOUSE_PASSWORD": "otel"},
			WaitingFor: wait.ForListeningPort("9000/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "9000")
	require.NoError(t, err)
	addr := host + ":" + port.Port()

	var args chcomponent.Arguments
	args.SetToDefault()
	args.Endpoint = "tcp://" + addr
	args.Username = "default"
	args.Password = "otel"
	// Disable async_insert so the row is guaranteed visible as soon as
	// ConsumeLogs returns, instead of depending on the server's flush timing.
	args.AsyncInsert = false
	// Disable the sending queue so ConsumeLogs pushes synchronously instead of
	// enqueueing for a background consumer goroutine to deliver later.
	args.Queue.Enabled = false

	cfg, err := args.Convert()
	require.NoError(t, err)

	fact := clickhouseexporter.NewFactory()
	exp, err := fact.CreateLogs(ctx, exportertest.NewNopSettings(fact.Type()), cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(ctx, nil))
	t.Cleanup(func() { _ = exp.Shutdown(ctx) })

	require.NoError(t, exp.ConsumeLogs(ctx, oneLogRecord("hello from otelcol.exporter.clickhouse")))

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{Username: "default", Password: "otel"},
	})
	require.NoError(t, err)
	defer conn.Close()

	var (
		count uint64
		body  string
	)
	require.NoError(t, conn.QueryRow(ctx, "SELECT count(), any(Body) FROM otel_logs").Scan(&count, &body))
	require.Equal(t, uint64(1), count)
	require.Equal(t, "hello from otelcol.exporter.clickhouse", body)
}

func oneLogRecord(body string) plog.Logs {
	logs := plog.NewLogs()
	record := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	record.Body().SetStr(body)
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return logs
}
