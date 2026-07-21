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

	// Verify prometheus_export
	prometheus, ok := config["prometheus_export"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 12345, prometheus["port"])
	require.Equal(t, []any{"application"}, prometheus["features"])
	require.Equal(t, []any{"*"}, prometheus["instrumentations"])

	// Verify discovery
	discovery, ok := config["discovery"].(map[string]any)
	require.True(t, ok)
	survey, ok := discovery["survey"].([]any)
	require.True(t, ok)
	require.Len(t, survey, 1)
	surveyItem := survey[0].(map[string]any)
	require.Equal(t, ".*testserver.*", surveyItem["exe_path"])

	// Verify ebpf
	ebpf, ok := config["ebpf"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "disabled", ebpf["context_propagation"])
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

	export, ok := config["otel_traces_export"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "unix://beyla-otlp-test", export["endpoint"])
	require.Equal(t, []any{"http"}, export["instrumentations"])
	require.Equal(t, "always_on", export["sampler"].(map[string]any)["name"])
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

	// Verify newly generated EBPF fields round-trip correctly.
	ebpf := config["ebpf"].(map[string]any)
	require.Equal(t, 64, ebpf["batch_length"])
	require.Equal(t, "5s", ebpf["batch_timeout"])
	require.Equal(t, 128, ebpf["couchbase_db_cache_size"])

	bufSizes := ebpf["buffer_sizes"].(map[string]any)
	require.Equal(t, 1024, bufSizes["http"])

	// Verify inject_wrapper: openai/anthropic/gemini/bedrock nested under genai.
	http := ebpf["payload_extraction"].(map[string]any)["http"].(map[string]any)
	genai := http["genai"].(map[string]any)
	require.Equal(t, true, genai["gemini"].(map[string]any)["enabled"])
	// Direct http field (not wrapped).
	require.Equal(t, true, http["graphql"].(map[string]any)["enabled"])

	// Verify new Stats fields.
	stats := config["stats"].(map[string]any)
	reverseDNS := stats["reverse_dns"].(map[string]any)
	require.Equal(t, 512, reverseDNS["cache_len"])
	require.Equal(t, "local", reverseDNS["type"])

	// Verify new Discovery fields.
	disc := config["discovery"].(map[string]any)
	require.Equal(t, true, disc["bpf_pid_filter_off"])
	require.Equal(t, []any{"/usr/lib"}, disc["excluded_linux_system_paths"])
	require.Equal(t, "30s", disc["min_process_age"])
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

	// Verify network
	networkFlows, ok := config["network"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, networkFlows["enable"])
	require.Equal(t, "0.0.0.0", networkFlows["agent_ip"])
	require.Equal(t, []any{"eth0"}, networkFlows["interfaces"])
	require.Equal(t, []any{"TCP", "UDP"}, networkFlows["protocols"])
	require.Equal(t, 1, networkFlows["sampling"])
	require.Equal(t, []any{"10.0.0.0/8"}, networkFlows["cidrs"])
	require.Equal(t, "ingress", networkFlows["direction"])
	require.Equal(t, "ipv4", networkFlows["agent_ip_type"])

	// geo_ip/reverse_dns must survive under network (backward compat with released beyla.ebpf).
	geoIP := networkFlows["geo_ip"].(map[string]any)
	require.Equal(t, 1024, geoIP["cache_len"])
	require.Equal(t, "/etc/geoip/country.mmdb", geoIP["maxmind"].(map[string]any)["country_path"])
	reverseDNS := networkFlows["reverse_dns"].(map[string]any)
	require.Equal(t, "local", reverseDNS["type"])
	require.Equal(t, 256, reverseDNS["cache_len"])
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

	injector, ok := config["injector"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"java"}, injector["enabled_sdks"])
	require.Equal(t, "http://alloy:4318", injector["exporter_otlp_endpoint"])
}

func TestYAMLGeneration_InternalMetricsDefault(t *testing.T) {
	// With nothing configured, Beyla's beyla_internal_* metrics must still be
	// exposed on the scraped /metrics endpoint (parity with in-process Beyla).
	config := buildYAML(t, Arguments{}, Runtime{Port: 12345})

	im, ok := config["internal_metrics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "prometheus", im["exporter"])
	prom := im["prometheus"].(map[string]any)
	require.Equal(t, 12345, prom["port"])
	require.Equal(t, "/metrics", prom["path"])

	// An explicit exporter overrides the default and drops the prometheus block.
	config = buildYAML(t, Arguments{InternalMetrics: InternalMetrics{Exporter: "disabled"}}, Runtime{Port: 12345})
	im = config["internal_metrics"].(map[string]any)
	require.Equal(t, "disabled", im["exporter"])
	require.NotContains(t, im, "prometheus")
}
