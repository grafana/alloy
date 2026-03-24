package receiver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/auth/headers"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	otlphttp "github.com/grafana/alloy/internal/component/otelcol/exporter/otlphttp"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

func TestWithOtelcolConsumer(t *testing.T) {
	ctx := componenttest.TestContext(t)

	faroReceiver, err := componenttest.NewControllerFromID(
		util.TestLogger(t),
		"faro.receiver",
	)
	require.NoError(t, err)
	faroReceiverPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	otelcolAuthHeader, err := componenttest.NewControllerFromID(
		util.TestLogger(t),
		"otelcol.auth.headers",
	)
	require.NoError(t, err)

	otelcolExporter, err := componenttest.NewControllerFromID(
		util.TestLogger(t),
		"otelcol.exporter.otlphttp",
	)
	require.NoError(t, err)

	doneChan := make(chan struct{})
	finalOtelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "TENANTID", r.Header.Get("X-Scope-OrgId"))
		close(doneChan)
		w.WriteHeader(http.StatusOK)
	}))
	defer finalOtelServer.Close()

	tenantId := "Tenant-Id"
	go func() {
		err := otelcolAuthHeader.Run(ctx, headers.Arguments{
			Headers: []headers.Header{
				{
					Key:         "X-Scope-OrgId",
					FromContext: &tenantId,
					Action:      headers.ActionUpsert,
				},
			},
			DebugMetrics: otelcolCfg.DebugMetricsArguments{
				Level: otelcolCfg.LevelDetailed,
			},
		})
		require.NoError(t, err)
	}()

	require.NoError(t, otelcolAuthHeader.WaitRunning(time.Second), "otelcol.auth.headers never started")
	require.NoError(t, otelcolAuthHeader.WaitExports(time.Second), "otelcol.auth.headers never exported anything")
	otelcolAuthHeaderExport, ok := otelcolAuthHeader.Exports().(auth.Exports)
	require.True(t, ok)

	// Parse exporter config from Alloy syntax to ensure defaults are properly initialized
	exporterCfg := fmt.Sprintf(`
		client {
			endpoint = "%s"
			tls {
				insecure             = true
				insecure_skip_verify = true
			}
		}
		encoding = "json"
		`, finalOtelServer.URL)
	var exporterArgs otlphttp.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(exporterCfg), &exporterArgs))
	// Set the auth handler after unmarshalling since it's a dynamic reference
	exporterArgs.Client.Authentication = otelcolAuthHeaderExport.Handler

	go func() {
		err := otelcolExporter.Run(ctx, exporterArgs)
		require.NoError(t, err)
	}()

	require.NoError(t, otelcolExporter.WaitRunning(time.Second), "otelco.exporter.otlphttp never started")
	require.NoError(t, otelcolExporter.WaitExports(time.Second), "otelco.exporter.otlphttp never exported anything")
	otelcolExporterExport, ok := otelcolExporter.Exports().(otelcol.ConsumerExports)
	require.True(t, ok)

	go func() {
		err := faroReceiver.Run(ctx, Arguments{
			LogLabels: map[string]string{
				"foo": "bar",
			},

			Server: ServerArguments{
				Host:            "127.0.0.1",
				Port:            faroReceiverPort,
				IncludeMetadata: true,
			},

			Output: OutputArguments{
				Traces: []otelcol.Consumer{otelcolExporterExport.Input},
			},
		})
		require.NoError(t, err)
	}()

	// Wait for the server to be running.
	util.Eventually(t, func(t require.TestingT) {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/-/ready", faroReceiverPort))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Send a sample payload to the server.
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("http://localhost:%d/collect", faroReceiverPort),
		strings.NewReader(`{
			"traces": {
				"resourceSpans": [{
					"scope_spans": [{
						"spans": [{
							"name": "TestSpan"
						}]
					}]
				}]
			},
			"logs": [{
				"message": "hello, world",
				"level": "info",
				"context": {"env": "dev"},
				"timestamp": "2021-01-01T00:00:00Z",
				"trace": {
					"trace_id": "0",
					"span_id": "0"
				}
			}],
			"exceptions": [],
			"measurements": [],
			"meta": {}
		}`),
	)
	require.NoError(t, err)

	req.Header.Add(tenantId, "TENANTID")
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	select {
	case <-doneChan:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for updates to finish")
	}
}
