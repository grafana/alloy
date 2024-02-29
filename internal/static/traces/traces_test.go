package traces

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/server"
	"github.com/grafana/agent/internal/static/traces/traceutils"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/dskit/log"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v2"
)

func TestTraces(t *testing.T) {
	tracesCh := make(chan ptrace.Traces)
	tracesAddr := traceutils.NewTestServer(t, func(t ptrace.Traces) {
		tracesCh <- t
	})

	tracesCfgText := util.Untab(fmt.Sprintf(`
configs:
- name: default
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

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(tracesCfgText))
	dec.SetStrict(true)
	err := dec.Decode(&cfg)
	require.NoError(t, err)

	var loggingLevel log.Level
	require.NoError(t, loggingLevel.Set("debug"))

	traces, err := New(nil, nil, prometheus.NewRegistry(), cfg, &server.HookLogger{})
	require.NoError(t, err)
	t.Cleanup(traces.Stop)

	tr := testJaegerTracer(t)
	span := tr.StartSpan("test-span")
	span.Finish()

	select {
	case <-time.After(30 * time.Second):
		require.Fail(t, "failed to receive a span after 30 seconds")
	case tr := <-tracesCh:
		require.Equal(t, 1, tr.SpanCount())
		// Nothing to do, send succeeded.
	}
}

func TestTraceWithSpanmetricsConfig(t *testing.T) {
	tracesCfgText := util.Untab(`
configs:
- name: test
  receivers:
    zipkin:
      endpoint: 0.0.0.0:9999
  remote_write:
    - endpoint: 0.0.0.0:5555
      insecure: false
      tls_config:
          insecure_skip_verify: true
  spanmetrics:
    handler_endpoint: 0.0.0.0:9090
    const_labels:
      key1: "value1"
      key2: "value2"
	`)

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(tracesCfgText))
	dec.SetStrict(true)
	err := dec.Decode(&cfg)
	require.NoError(t, err)

	var loggingLevel log.Level
	require.NoError(t, loggingLevel.Set("debug"))

	traces, err := New(nil, nil, prometheus.NewRegistry(), cfg, &server.HookLogger{})
	require.NoError(t, err)
	t.Cleanup(traces.Stop)
}

func TestTrace_ApplyConfig(t *testing.T) {
	tracesCh := make(chan ptrace.Traces)
	tracesAddr := traceutils.NewTestServer(t, func(t ptrace.Traces) {
		tracesCh <- t
	})

	tracesCfgText := util.Untab(`
configs:
- name: default
  receivers:
    jaeger:
      protocols:
        thrift_compact:
  remote_write:
  	- endpoint: 127.0.0.1:80 # deliberately the wrong endpoint
  	  insecure: true
  batch:
    timeout: 100ms
    send_batch_size: 1
  service_graphs:
    enabled: true
`)

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(tracesCfgText))
	dec.SetStrict(true)
	err := dec.Decode(&cfg)
	require.NoError(t, err)

	traces, err := New(nil, nil, prometheus.NewRegistry(), cfg, &server.HookLogger{})
	require.NoError(t, err)
	t.Cleanup(traces.Stop)

	// Fix the config and apply it before sending spans.
	tracesCfgText = util.Untab(fmt.Sprintf(`
configs:
- name: default
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

	var fixedConfig Config
	dec = yaml.NewDecoder(strings.NewReader(tracesCfgText))
	dec.SetStrict(true)
	err = dec.Decode(&fixedConfig)
	require.NoError(t, err)

	err = traces.ApplyConfig(nil, nil, fixedConfig)
	require.NoError(t, err)

	tr := testJaegerTracer(t)
	span := tr.StartSpan("test-span")
	span.Finish()

	select {
	case <-time.After(30 * time.Second):
		require.Fail(t, "failed to receive a span after 30 seconds")
	case tr := <-tracesCh:
		require.Equal(t, 1, tr.SpanCount())
		// Nothing to do, send succeeded.
	}
}

func testJaegerTracer(t *testing.T) opentracing.Tracer {
	t.Helper()

	jaegerConfig := jaegercfg.Configuration{
		ServiceName: "TestTraces",
		Sampler: &jaegercfg.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LocalAgentHostPort: "127.0.0.1:6831",
			LogSpans:           true,
		},
	}
	tr, closer, err := jaegerConfig.NewTracer()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closer.Close())
	})

	return tr
}
