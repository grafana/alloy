package app_agent_receiver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/grafana/agent/internal/static/server"
	"github.com/grafana/agent/internal/static/traces"
	"github.com/grafana/agent/internal/static/traces/traceutils"
	"github.com/grafana/agent/internal/util"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v2"
)

func Test_ReceiveTracesAndRemoteWrite(t *testing.T) {
	var err error

	//
	// Prepare the traces instance
	//
	tracesCh := make(chan ptrace.Traces)
	tracesAddr := traceutils.NewTestServer(t, func(t ptrace.Traces) {
		tracesCh <- t
	})

	tracesCfgText := util.Untab(fmt.Sprintf(`
configs:
- name: TEST_TRACES
  receivers:
    jaeger:
      protocols:
        thrift_compact:
  remote_write:
  	- endpoint: %s
      insecure: true
  batch:
    timeout: 100ms
    send_batch_size: 1
	`, tracesAddr))

	var tracesCfg traces.Config
	dec := yaml.NewDecoder(strings.NewReader(tracesCfgText))
	dec.SetStrict(true)
	err = dec.Decode(&tracesCfg)
	require.NoError(t, err)

	traces, err := traces.New(nil, nil, prometheus.NewRegistry(), tracesCfg, &server.HookLogger{})
	require.NoError(t, err)
	t.Cleanup(traces.Stop)

	//
	// Prepare the app_agent_receiver integration
	//
	integrationPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	var integrationCfg Config
	cb := fmt.Sprintf(`
instance: TEST_APP_AGENT_RECEIVER
server:
  cors_allowed_origins:
  - '*'
  host: '0.0.0.0'
  max_allowed_payload_size: 5e+07
  port: %d
  rate_limiting:
  burstiness: 100
  enabled: true
  rps: 100
sourcemaps:
  download: true
traces_instance: TEST_TRACES
`, integrationPort)
	err = yaml.Unmarshal([]byte(cb), &integrationCfg)
	require.NoError(t, err)

	logger := util.TestLogger(t)
	globals := integrations.Globals{
		Tracing: traces,
	}

	integration, err := integrationCfg.NewIntegration(logger, globals)
	require.NoError(t, err)

	ctx := context.Background()
	t.Cleanup(func() { ctx.Done() })
	//
	// Start the app_agent_receiver integration
	//
	go func() {
		err = integration.RunIntegration(ctx)
		require.NoError(t, err)
	}()

	//
	// Send data to the integration's /collect endpoint
	//
	const PAYLOAD = `
{
  "traces": {
    "resourceSpans": [{
		"scopeSpans": [{
			"spans": [{
				"name": "TestSpan",
				"attributes": [{
					"key": "foo",
					"value": { "intValue": "11111" }
				},
				{
					"key": "boo",
					"value": { "intValue": "22222" }
				},
				{
					"key": "user.email",
					"value": { "stringValue": "user@email.com" }
				}]
			}]
		}]
	}]
  },
  "logs": [],
  "exceptions": [],
  "measurements": [],
  "meta": {}
}
`

	integrationURL := fmt.Sprintf("http://127.0.0.1:%d/collect", integrationPort)

	var httpResponse *http.Response
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		req, err := http.NewRequest("POST", integrationURL, bytes.NewBuffer([]byte(PAYLOAD)))
		assert.NoError(c, err)

		httpResponse, err = http.DefaultClient.Do(req)
		assert.NoError(c, err)
	}, 5*time.Second, 250*time.Millisecond)

	//
	// Check that the data was received by the integration
	//
	resBody, err := io.ReadAll(httpResponse.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(resBody[:]))

	require.Equal(t, http.StatusAccepted, httpResponse.StatusCode)

	//
	// Check that the traces subsystem remote wrote the integration
	//
	select {
	case <-time.After(10 * time.Second):
		require.Fail(t, "failed to receive a span after 10 seconds")
	case tr := <-tracesCh:
		require.Equal(t, 1, tr.SpanCount())
		// Nothing to do, send succeeded.
	}
}
