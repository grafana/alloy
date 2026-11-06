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

	"github.com/grafana/alloy/syntax"
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
			Survey: SurveyServices{
				{
					Service: Service{Path: ".*testserver.*"},
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

func TestYAMLGeneration_ExemplarFilterDefault(t *testing.T) {
	// Beyla logs a warning when exemplar_filter is empty, so Alloy must always
	// emit an explicit value. Unset means the documented default, always_off.
	config := buildYAML(t, Arguments{}, Runtime{Port: 12345})

	prom, ok := config["prometheus_export"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "always_off", prom["exemplar_filter"])

	// An explicit value is passed through unchanged.
	config = buildYAML(t, Arguments{Metrics: Metrics{ExemplarFilter: "trace_based"}}, Runtime{Port: 12345})
	prom = config["prometheus_export"].(map[string]any)
	require.Equal(t, "trace_based", prom["exemplar_filter"])
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

func TestYAMLGeneration_Beyla338Options(t *testing.T) {
	// Decode Alloy syntax and inspect the YAML Beyla receives so this covers
	// both the public config names and their translation.
	const input = `
		discovery {
			instrument { open_ports = "8080" }
			disabled_route_harvesters = ["dotnet", "php", "python", "ruby"]
		}
		ebpf {
			populate_trace_context = true
			kafka_consumer_group_cache_size = 8192
			kafka_consumer_group_ttl = "6m"
		}
		dotnet_runtime_metrics {
			sampling_interval = "2s"
			timeout = "15s"
		}
		cloud_metadata {
			cluster_name = "ecs-production"
			region = "ca-central-1"
		}
		name_resolver {
			sources = ["k8s", "rdns", "ecs"]
			cache_len = 2048
			cache_expiry = "10m"
		}
		metrics {
			features = ["application_red", "application_sizes", "application_runtime", "stats_tcp_successful_connections"]
			buckets {
				jvm_gc_duration_histogram = [0.02, 0.2, 2.5]
				v8js_gc_duration_histogram = [0.005, 0.05, 0.5]
			}
		}
		injector { enabled_sdks = ["ruby"] }
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(input), &args))
	cfg := buildYAML(t, args, Runtime{Port: 12345})
	require.Equal(t, map[string]any{
		"populate_trace_context":          true,
		"kafka_consumer_group_cache_size": 8192,
		"kafka_consumer_group_ttl":        "6m0s",
	}, cfg["ebpf"])
	require.Equal(t, map[string]any{"sampling_interval": "2s", "timeout": "15s"}, cfg["dotnet_runtime_metrics"])
	require.Equal(t, map[string]any{"cluster_name": "ecs-production", "region": "ca-central-1"}, cfg["cloud_metadata"])
	require.Equal(t, map[string]any{
		"sources":      []any{"k8s", "rdns", "ecs"},
		"cache_len":    2048,
		"cache_expiry": "10m0s",
	}, cfg["name_resolver"])
	prom := cfg["prometheus_export"].(map[string]any)
	require.Equal(t, []any{"application_red", "application_sizes", "application_runtime", "stats_tcp_successful_connections"}, prom["features"])
	require.Equal(t, []any{0.02, 0.2, 2.5}, prom["buckets"].(map[string]any)["jvm_gc_duration_histogram"])
	require.Equal(t, []any{0.005, 0.05, 0.5}, prom["buckets"].(map[string]any)["v8js_gc_duration_histogram"])
	require.Equal(t, []any{"dotnet", "php", "python", "ruby"}, cfg["discovery"].(map[string]any)["disabled_route_harvesters"])
	require.Equal(t, []any{"ruby"}, cfg["injector"].(map[string]any)["enabled_sdks"])
}

func TestYAMLGeneration_Beyla341Options(t *testing.T) {
	const input = `
		attributes {
			kubernetes { enable = "false" }
			sanitize_sql_query_text = true
		}
		discovery {
			instrument_all_local = true
			survey {
				exe_path = "/opt/batch/*"
			}
			survey {
				socket_apps {
					enabled  = true
					non_root = true
				}
			}
		}
		ebpf {
			payload_extraction {
				http {
					cohere { enabled = true }
				}
			}
		}
		cloud_metadata {
			refresh_interval = "1m"
			route53 {
				hosted_zone_ids  = ["Z0123456789EXAMPLE"]
				refresh_interval = "10m"
			}
		}
		name_resolver {
			sources = ["k8s", "rdns", "route53", "gce", "azure"]
			gce {
				project_id = "production-project"
				zone       = "europe-west3-a"
				endpoint   = "http://localhost:8080"
			}
			azure {
				subscription_id = "00000000-0000-0000-0000-000000000000"
				resource_group  = "production"
			}
		}
		metrics {
			features = ["application", "application_service_graph_connection_info"]
		}
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(input), &args))
	require.NoError(t, args.Validate())
	cfg := buildYAML(t, args, Runtime{Port: 12345})

	require.Equal(t, true, cfg["attributes"].(map[string]any)["sanitize_sql_query_text"])
	discovery := cfg["discovery"].(map[string]any)
	require.Equal(t, true, discovery["instrument_all_local"])
	require.Equal(t, []any{
		map[string]any{"exe_path": "/opt/batch/*"},
		map[string]any{"socket_apps": map[string]any{"enabled": true, "non_root": true}},
	}, discovery["survey"])
	genai := cfg["ebpf"].(map[string]any)["payload_extraction"].(map[string]any)["http"].(map[string]any)["genai"]
	require.Equal(t, map[string]any{"cohere": map[string]any{"enabled": true}}, genai)
	require.Equal(t, map[string]any{
		"refresh_interval": "1m0s",
		"route53": map[string]any{
			"hosted_zone_ids":  []any{"Z0123456789EXAMPLE"},
			"refresh_interval": "10m0s",
		},
	}, cfg["cloud_metadata"])
	require.Equal(t, map[string]any{
		"sources": []any{"k8s", "rdns", "route53", "gce", "azure"},
		"gce": map[string]any{
			"project_id": "production-project",
			"zone":       "europe-west3-a",
			"endpoint":   "http://localhost:8080",
		},
		"azure": map[string]any{
			"subscription_id": "00000000-0000-0000-0000-000000000000",
			"resource_group":  "production",
		},
	}, cfg["name_resolver"])
	require.Equal(t, []any{"application", "application_service_graph_connection_info"}, cfg["prometheus_export"].(map[string]any)["features"])
}

// Beyla v3.41.0 moved the ECS inventory refresh interval from name_resolver.ecs to
// cloud_metadata, where it ignores unknown keys. The deprecated Alloy setting must
// land where Beyla reads it.
func TestYAMLGeneration_DeprecatedECSRefreshInterval(t *testing.T) {
	const input = `
		cloud_metadata { region = "ca-central-1" }
		name_resolver {
			sources = ["ecs"]
			ecs { refresh_interval = "45s" }
		}
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(input), &args))
	require.NoError(t, args.Validate())
	cfg := buildYAML(t, args, Runtime{Port: 12345})
	require.Equal(t, map[string]any{"region": "ca-central-1", "refresh_interval": "45s"}, cfg["cloud_metadata"])
	require.Equal(t, map[string]any{"sources": []any{"ecs"}}, cfg["name_resolver"])
}

func TestYAMLGeneration_Beyla338Defaults(t *testing.T) {
	// An empty block must not overwrite Beyla's own defaults with zero values.
	const input = `
		ebpf { populate_trace_context = false }
		dotnet_runtime_metrics {}
		cloud_metadata { route53 {} }
		name_resolver {
			ecs {}
			gce {}
			azure {}
		}
		metrics { buckets {} }
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(input), &args))
	cfg := buildYAML(t, args, Runtime{Port: 12345})
	for _, section := range []string{"ebpf", "dotnet_runtime_metrics", "cloud_metadata", "name_resolver"} {
		require.NotContains(t, cfg, section)
	}
	require.NotContains(t, cfg["prometheus_export"], "buckets")
}

func TestGoHTTPClientBufferTimeout(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{"unset uses Beyla default", `ebpf {}`, "", ""},
		{"zero disables deferral", `ebpf { go_http_client_buffer_timeout = "0s" }`, "0s", ""},
		{"custom timeout", `ebpf { go_http_client_buffer_timeout = "250ms" }`, "250ms", ""},
		{"negative timeout", `ebpf { go_http_client_buffer_timeout = "-1s" }`, "", "ebpf.go_http_client_buffer_timeout must be greater than or equal to 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tt.input), &args)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			cfg := buildYAML(t, args, Runtime{Port: 12345})
			if tt.want == "" {
				require.NotContains(t, cfg, "ebpf")
			} else {
				require.Equal(t, tt.want, cfg["ebpf"].(map[string]any)["go_http_client_buffer_timeout"])
			}
		})
	}
}

func TestYAMLGeneration_PeerMetadata(t *testing.T) {
	const input = `
		peer_metadata {
			enabled            = true
			mode               = "peer"
			listen_address     = "10.0.0.5"
			port               = 9998
			peer_cidrs         = ["10.0.0.0/16"]
			refresh_interval   = "20s"
			caller_ttl         = "2m"
			cache_ttl          = "3m"
			request_timeout    = "1s"
			max_peers          = 64
			max_concurrent     = 4
			max_caller_entries = 1000
			max_response_bytes = 2097152
			max_cache_bytes    = 134217728
			token              = "test-token"
			tls {
				enabled     = true
				cert_file   = "/etc/beyla/tls.crt"
				key_file    = "/etc/beyla/tls.key"
				ca_file     = "/etc/beyla/ca.crt"
				server_name = "beyla.internal"
			}
		}
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(input), &args))
	require.NoError(t, args.Validate())
	cfg := buildYAML(t, args, Runtime{Port: 12345})
	require.Equal(t, map[string]any{
		"enabled":            true,
		"mode":               "peer",
		"listen_address":     "10.0.0.5",
		"port":               9998,
		"peer_cidrs":         []any{"10.0.0.0/16"},
		"refresh_interval":   "20s",
		"caller_ttl":         "2m0s",
		"cache_ttl":          "3m0s",
		"request_timeout":    "1s",
		"max_peers":          64,
		"max_concurrent":     4,
		"max_caller_entries": 1000,
		"max_response_bytes": 2097152,
		"max_cache_bytes":    134217728,
		"token":              "test-token",
		"tls": map[string]any{
			"enabled":     true,
			"cert_file":   "/etc/beyla/tls.crt",
			"key_file":    "/etc/beyla/tls.key",
			"ca_file":     "/etc/beyla/ca.crt",
			"server_name": "beyla.internal",
		},
	}, cfg["peer_metadata"])
}

func TestRedacted(t *testing.T) {
	args := Arguments{PeerMetadata: PeerMetadata{Enabled: true, Token: "test-token"}}
	cfg := Build(args, Runtime{Port: 12345}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	redacted := Redacted(cfg)
	require.Equal(t, "(secret)", redacted["peer_metadata"].(map[string]any)["token"])
	require.Equal(t, true, redacted["peer_metadata"].(map[string]any)["enabled"])
	data, err := yaml.Marshal(redacted)
	require.NoError(t, err)
	require.NotContains(t, string(data), "test-token")

	// The config passed to Beyla keeps the token.
	require.Equal(t, "test-token", cfg["peer_metadata"].(map[string]any)["token"])

	// A config without secrets is returned as is.
	plain := Build(Arguments{}, Runtime{Port: 12345}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Equal(t, plain, Redacted(plain))
}
