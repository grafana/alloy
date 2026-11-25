//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
)

type logsConsumer struct {
	otelcol.Consumer
	records chan plog.Logs
}

func (c *logsConsumer) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	c.records <- logs
	return nil
}

func TestOTLPLogsOnly(t *testing.T) {
	for _, encoding := range []string{"application/json", "application/x-protobuf"} {
		t.Run(encoding, func(t *testing.T) {
			sink := &logsConsumer{records: make(chan plog.Logs, 1)}
			c := &Component{opts: component.Options{ID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, args: Arguments{Output: &otelcol.ConsumerArguments{Logs: []otelcol.Consumer{sink}}}}
			require.NoError(t, c.startOTLPReceiver())
			defer c.stopOTLPReceiver()
			req := plogotlp.NewExportRequest()
			record := req.Logs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			record.Body().SetStr("dogfood error")
			record.Attributes().PutStr("exception.type", "error")
			var body []byte
			var err error
			if encoding == "application/json" {
				body, err = req.MarshalJSON()
			} else {
				body, err = req.MarshalProto()
			}
			require.NoError(t, err)
			transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", c.otlpReceiverAddr)
			}}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			response, err := client.Post("http://localhost/v1/logs", encoding, bytes.NewReader(body))
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)
			select {
			case logs := <-sink.records:
				got := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
				require.Equal(t, "dogfood error", got.Body().Str())
				attr, ok := got.Attributes().Get("exception.type")
				require.True(t, ok)
				require.Equal(t, "error", attr.Str())
			case <-time.After(5 * time.Second):
				t.Fatal("OTLP log was not forwarded")
			}
		})
	}
}
