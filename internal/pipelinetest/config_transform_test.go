package pipelinetest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const transformSrc = `// comment kept
loki.source.file "app" {
  targets    = []
  forward_to = [loki.process.app.receiver, loki.write.cloud.receiver]
}

loki.process "app" {
  forward_to = [loki.write.cloud.receiver]
}

loki.write "cloud" {
  endpoint {
    url = "https://example.com"
  }
  external_labels = {cluster = sys.env("CLUSTER")}
}

discovery.kubernetes "pods" {
  role = "pod"
}
`

func TestTransformConfig(t *testing.T) {
	out, err := transformConfig("config.alloy", transformSrc, transformSpec{
		sources: []sourceReplacement{{id: "loki.source.file.app", signal: signalLogs}},
		sinks:   []string{"loki.write.cloud"},
		remove:  []string{"discovery.kubernetes.pods"},
	})
	require.NoError(t, err)

	require.Equal(t, "[loki.process.app.receiver, pipelinetest.sink.loki_write_cloud.loki_receiver]", out.sources["loki.source.file.app"].forwardTo)
	require.Equal(t, map[string]string{"loki.write.cloud.receiver": "pipelinetest.sink.loki_write_cloud.loki_receiver"}, out.sinkRefs)
	require.Equal(t, []generatedSink{{exit: "loki.write.cloud", label: "loki_write_cloud", externalLabels: `{cluster = sys.env("CLUSTER")}`}}, out.sinks)

	require.Contains(t, out.source, "// comment kept")
	require.Contains(t, out.source, "forward_to = [pipelinetest.sink.loki_write_cloud.loki_receiver]")
	require.NotContains(t, out.source, "loki.source.file")
	require.NotContains(t, out.source, "loki.write")
	require.NotContains(t, out.source, "discovery.kubernetes")

	// Removed blocks leave their newlines behind, so the line of every kept
	// component is unchanged.
	require.Equal(t, strings.Count(transformSrc, "\n"), strings.Count(out.source, "\n"))
	require.Equal(t, lineOf(transformSrc, `loki.process "app"`), lineOf(out.source, `loki.process "app"`))
}

func TestTransformConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		sources []sourceReplacement
		sinks   []string
		remove  []string
		wantErr string
	}{
		{
			name:    "unknown component",
			sinks:   []string{"loki.write.missing"},
			wantErr: "loki.write.missing to replace with the sink is not a top-level block",
		},
		{
			name:    "unsupported sink",
			sinks:   []string{"loki.process.app"},
			wantErr: "only loki.write, prometheus.remote_write and otelcol.exporter.* components can be replaced with the sink",
		},
		{
			name:    "listed twice",
			sinks:   []string{"loki.write.cloud"},
			remove:  []string{"loki.write.cloud"},
			wantErr: "component loki.write.cloud is listed twice",
		},
		{
			name:    "source without forward_to",
			sources: []sourceReplacement{{id: "discovery.kubernetes.pods", signal: signalLogs}},
			wantErr: "discovery.kubernetes.pods has no forward_to attribute",
		},
		{
			name:    "traces source without output",
			sources: []sourceReplacement{{id: "loki.source.file.app", signal: signalTraces}},
			wantErr: "loki.source.file.app has no output block",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := transformConfig("config.alloy", transformSrc, transformSpec{sources: tt.sources, sinks: tt.sinks, remove: tt.remove})
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestTransformConfigRejectsOtherSinkExports(t *testing.T) {
	src := `otelcol.exporter.otlp "tempo" {
  client { endpoint = "tempo:4317" }
}

otelcol.processor.batch "default" {
  output {
    traces = [otelcol.exporter.otlp.tempo.input]
  }
}

local.file "x" {
  filename = otelcol.exporter.otlp.tempo.something
}
`
	_, err := transformConfig("config.alloy", src, transformSpec{sinks: []string{"otelcol.exporter.otlp.tempo"}})
	require.ErrorContains(t, err, "references otelcol.exporter.otlp.tempo.something")
}

func lineOf(src, substr string) int {
	return strings.Count(src[:strings.Index(src, substr)], "\n") + 1
}

func TestTransformConfigOverride(t *testing.T) {
	out, err := transformConfig("config.alloy", transformSrc, transformSpec{
		sinks:     []string{"loki.write.cloud"},
		overrides: map[string]map[string]string{"discovery.kubernetes.pods": {"role": `"node"`}},
	})
	require.NoError(t, err)
	require.Contains(t, out.source, `role = "node"`)
	require.Equal(t, strings.Count(transformSrc, "\n"), strings.Count(out.source, "\n"))

	_, err = transformConfig("config.alloy", transformSrc, transformSpec{
		overrides: map[string]map[string]string{"discovery.kubernetes.pods": {"namespaces": `[]`}},
	})
	require.ErrorContains(t, err, "does not set namespaces")

	_, err = transformConfig("config.alloy", transformSrc, transformSpec{
		sinks:     []string{"loki.write.cloud"},
		overrides: map[string]map[string]string{"loki.write.cloud": {"x": `1`}},
	})
	require.ErrorContains(t, err, "cannot be overridden")
}
