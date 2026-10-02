//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// buildYAML runs Build and round-trips through YAML so assertions see the same
// homogenized types (e.g. []any) Beyla parses from the on-disk config.
func buildYAML(t *testing.T, args Arguments, rt Runtime) map[string]any {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	data, err := yaml.Marshal(Build(args, rt, log))
	require.NoError(t, err)
	var config map[string]any
	require.NoError(t, yaml.Unmarshal(data, &config))
	return config
}

// mustYAML marshals v back to a YAML string for require.YAMLEq comparisons.
func mustYAML(t *testing.T, v any) string {
	t.Helper()
	data, err := yaml.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func TestYAMLGeneration(t *testing.T) {
	args := Arguments{
		Discovery: Discovery{
			Survey: Services{
				{
					Path: ".*testserver.*",
				},
			},
		},
		Metrics: Metrics{
			Features:         []string{"application"},
			Instrumentations: []string{"*"},
		},
		EBPF: EBPF{
			ContextPropagation: "disabled",
		},
	}

	config := buildYAML(t, args, Runtime{Port: 12345})

	require.YAMLEq(t, `
port: 12345
features: [application]
instrumentations: ['*']
`, mustYAML(t, config["prometheus_export"]))

	require.YAMLEq(t, `
survey:
  - exe_path: .*testserver.*
`, mustYAML(t, config["discovery"]))

	require.YAMLEq(t, `
context_propagation: disabled
`, mustYAML(t, config["ebpf"]))
}

func TestYAMLGeneration_TracesExport(t *testing.T) {
	args := Arguments{
		Traces: Traces{
			Instrumentations: []string{"http"},
			Sampler:          SamplerConfig{Name: "always_on"},
		},
		Output: &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{nil}},
	}

	config := buildYAML(t, args, Runtime{Port: 12345, OTLPAddr: "beyla-otlp-test"})

	require.YAMLEq(t, `
endpoint: unix://beyla-otlp-test
protocol: http/protobuf
instrumentations: [http]
sampler:
  name: always_on
`, mustYAML(t, config["otel_traces_export"]))
	require.NotContains(t, config, "traces")
}

func TestYAMLGeneration_NewSchemaFields(t *testing.T) {
	args := Arguments{
		EBPF: EBPF{
			BatchLength:          64,
			BatchTimeout:         5 * time.Second,
			CouchbaseDbCacheSize: 128,
			BufferSizes: EBPFBufferSizes{
				Http: 1024,
			},
			PayloadExtraction: PayloadExtraction{
				HTTP: HTTPPayloadExtraction{
					Graphql: GraphQLConfig{Enabled: true},
					Gemini:  GeminiConfig{Enabled: true},
				},
			},
		},
		Stats: Stats{
			ReverseDns: ReverseDNS{
				CacheLen: 512,
				Type:     "local",
			},
		},
		Discovery: Discovery{
			BpfPidFilterOff:          true,
			ExcludedLinuxSystemPaths: []string{"/usr/lib"},
			MinProcessAge:            30 * time.Second,
		},
		Metrics: Metrics{Features: []string{"network"}},
	}

	config := buildYAML(t, args, Runtime{Port: 9090})

	// Verify newly generated EBPF fields round-trip correctly, including inject_wrapper:
	// openai/anthropic/gemini/bedrock nested under genai, while graphql stays a direct http field.
	require.YAMLEq(t, `
batch_length: 64
batch_timeout: 5s
couchbase_db_cache_size: 128
buffer_sizes:
  http: 1024
payload_extraction:
  http:
    graphql:
      enabled: true
    genai:
      gemini:
        enabled: true
`, mustYAML(t, config["ebpf"]))

	require.YAMLEq(t, `
reverse_dns:
  cache_len: 512
  type: local
`, mustYAML(t, config["stats"]))

	require.YAMLEq(t, `
bpf_pid_filter_off: true
excluded_linux_system_paths: [/usr/lib]
min_process_age: 30s
`, mustYAML(t, config["discovery"]))
}

func TestYAMLGeneration_NetworkFlows(t *testing.T) {
	args := Arguments{
		Metrics: Metrics{
			Features: []string{"network"},
			Network: Network{
				Enable:      true,
				AgentIP:     "0.0.0.0",
				Interfaces:  []string{"eth0"},
				Protocols:   []string{"TCP", "UDP"},
				Sampling:    1,
				CIDRs:       []string{"10.0.0.0/8"},
				Direction:   "ingress",
				AgentIPType: "ipv4",
				GeoIp: GeoIP{
					CacheLen: 1024,
					Maxmind:  MaxMindConfig{CountryPath: "/etc/geoip/country.mmdb"},
				},
				ReverseDns: ReverseDNS{
					Type:     "local",
					CacheLen: 256,
				},
			},
		},
	}

	config := buildYAML(t, args, Runtime{Port: 12345})

	// geo_ip/reverse_dns must survive under network (backward compat with released beyla.ebpf).
	require.YAMLEq(t, `
enable: true
agent_ip: 0.0.0.0
interfaces: [eth0]
protocols: [TCP, UDP]
sampling: 1
cidrs: [10.0.0.0/8]
direction: ingress
agent_ip_type: ipv4
geo_ip:
  cache_len: 1024
  maxmind:
    country_path: /etc/geoip/country.mmdb
reverse_dns:
  type: local
  cache_len: 256
`, mustYAML(t, config["network"]))
}

func TestYAMLGeneration_InjectorEnabledSDKs(t *testing.T) {
	// enabled_sdks (schema []InstrumentableType) is exposed as []string via the
	// scalar_types hint, and exporter_otlp_endpoint replaces Beyla 3.22's old otel_endpoint.
	args := Arguments{
		Injector: Injector{
			EnabledSdks:          []string{"java"},
			ExporterOtlpEndpoint: "http://alloy:4318",
		},
	}

	config := buildYAML(t, args, Runtime{Port: 4318})

	require.YAMLEq(t, `
enabled_sdks: [java]
exporter_otlp_endpoint: http://alloy:4318
`, mustYAML(t, config["injector"]))
}

func TestYAMLGeneration_InternalMetricsDefault(t *testing.T) {
	// With nothing configured, Beyla's beyla_internal_* metrics must still be
	// exposed on the scraped /metrics endpoint (parity with in-process Beyla).
	config := buildYAML(t, Arguments{}, Runtime{Port: 12345})

	require.YAMLEq(t, `
exporter: prometheus
prometheus:
  port: 12345
  path: /metrics
`, mustYAML(t, config["internal_metrics"]))

	// An explicit exporter overrides the default and drops the prometheus block.
	config = buildYAML(t, Arguments{InternalMetrics: InternalMetrics{Exporter: "disabled"}}, Runtime{Port: 12345})
	require.YAMLEq(t, `
exporter: disabled
`, mustYAML(t, config["internal_metrics"]))
}
