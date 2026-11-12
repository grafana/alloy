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

const moduleSrc = `declare "cluster_metrics" {
  argument "destinations" {}

  discovery.kubernetes "nodes" {
    role = "node"
  }

  discovery.relabel "nodes" {
    targets = discovery.kubernetes.nodes.targets
  }

  prometheus.scrape "nodes" {
    targets    = discovery.relabel.nodes.output
    forward_to = argument.destinations.value
  }

  local.file "token" {
    filename = "/var/run/token"
  }
}

cluster_metrics "a" {
  destinations = [prometheus.remote_write.cloud.receiver]
}

cluster_metrics "b" {
  destinations = [prometheus.remote_write.cloud.receiver]
}

discovery.kubernetes "nodes" {
  role = "node"
}

prometheus.scrape "top" {
  targets    = discovery.kubernetes.nodes.targets
  forward_to = [prometheus.remote_write.cloud.receiver]
}

prometheus.remote_write "cloud" {
  endpoint {
    url = "https://example.com"
    write_relabel_config {
      target_label = "scraped_by"
      replacement  = "alloy"
    }
  }
}
`

func TestTransformConfigModules(t *testing.T) {
	out, err := transformConfig("config.alloy", moduleSrc, transformSpec{
		sources:      []sourceReplacement{{id: "cluster_metrics/prometheus.scrape.nodes", signal: signalMetrics, name: "in"}},
		sinks:        []string{"prometheus.remote_write.cloud"},
		remove:       []string{"cluster_metrics/local.file.token"},
		discovery:    map[string]string{"cluster_metrics/discovery.kubernetes.nodes": `[{"__address__" = "a"}]`},
		writeRelabel: true,
	})
	require.NoError(t, err)

	// The input's source goes inside the module, once per instance.
	require.Equal(t, replacedSource{forwardTo: "argument.destinations.value", modules: []string{"cluster_metrics.a", "cluster_metrics.b"}}, out.sources["cluster_metrics/prometheus.scrape.nodes"])
	require.Contains(t, out.source, `pipelinetest.source "in" { forward_to { metrics = argument.destinations.value } }`)
	require.NotContains(t, out.source, "local.file")

	// Only the module's discovery.kubernetes.nodes is replaced. The top-level
	// one with the same ID stays.
	require.Contains(t, out.source, "targets = [{\"__address__\" = \"a\"}]")
	require.Contains(t, out.source, "targets    = discovery.kubernetes.nodes.targets\n  forward_to = [pipelinetest.sink")

	require.Equal(t, []string{"write_relabel_config {\n      target_label = \"scraped_by\"\n      replacement  = \"alloy\"\n    }"}, out.sinks[0].writeRelabel)

	require.Equal(t, strings.Count(moduleSrc, "\n"), strings.Count(out.source, "\n"))
	require.Equal(t, lineOf(moduleSrc, `cluster_metrics "a"`), lineOf(out.source, `cluster_metrics "a"`))
}

func TestTransformConfigModuleErrors(t *testing.T) {
	tests := []struct {
		name    string
		spec    transformSpec
		wantErr string
	}{
		{
			name:    "unknown module",
			spec:    transformSpec{remove: []string{"node_logs/local.file.token"}},
			wantErr: `the config has no declare block "node_logs"; modules: cluster_metrics`,
		},
		{
			name:    "unknown component in module",
			spec:    transformSpec{remove: []string{"cluster_metrics/local.file.missing"}},
			wantErr: "is not a component of module cluster_metrics",
		},
		{
			name:    "top-level ID does not reach into the module",
			spec:    transformSpec{remove: []string{"local.file.token"}},
			wantErr: "is not a top-level block of the config",
		},
		{
			name:    "sink in a module",
			spec:    transformSpec{sinks: []string{"cluster_metrics/local.file.token"}},
			wantErr: "exits inside modules can't be replaced yet",
		},
		{
			name:    "override in a module",
			spec:    transformSpec{overrides: map[string]map[string]string{"cluster_metrics/local.file.token": {"missing": `"x"`}}},
			wantErr: "component cluster_metrics/local.file.token does not set missing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := transformConfig("config.alloy", moduleSrc, tt.spec)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}

	out, err := transformConfig("config.alloy", moduleSrc, transformSpec{
		overrides: map[string]map[string]string{"cluster_metrics/local.file.token": {"filename": `"./token"`}},
	})
	require.NoError(t, err)
	require.Contains(t, out.source, `filename = "./token"`)
}

func TestTransformConfigWriteRelabelNeedsOneEndpoint(t *testing.T) {
	src := `prometheus.remote_write "cloud" {
  endpoint { url = "https://a.example.com" }
  endpoint { url = "https://b.example.com" }
}
`
	_, err := transformConfig("config.alloy", src, transformSpec{sinks: []string{"prometheus.remote_write.cloud"}, writeRelabel: true})
	require.ErrorContains(t, err, "write_relabel needs one endpoint block, the exit has 2")
}
