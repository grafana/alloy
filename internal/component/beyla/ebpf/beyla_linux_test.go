//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
)

func TestArguments_UnmarshalSyntax(t *testing.T) {
	in := `
		routes {
			unmatched = "wildcard"
			patterns = ["/api/v1/*"]
			ignored_patterns = ["/api/v1/health"]
			ignore_mode = "all"
			wildcard_char = "*"
		}
		debug = false
		attributes {
			kubernetes {
				enable = "true"
				informers_sync_timeout = "15s"
				informers_resync_period = "30m"
				cluster_name = "test"
				disable_informers = ["node"]
				meta_restrict_local_node = true
				meta_cache_address = "localhost:9090"
			}
			select {
				attr = "sql_client_duration"
				include = ["*"]
				exclude = ["db_statement"]
			}
		}
		discovery {
			instrument {
				name = "test"
				namespace = "default"
				open_ports = "80,443"
				exe_path = "/usr/bin/app*"
				kubernetes {
					namespace = "default"
				}
				exports = ["metrics", "traces"]
				sampler {
					name = "traceidratio"
					arg = "0.5"
				}
			}
			instrument {
				name = "test2"
				namespace = "default"
				open_ports = "8080"
				exe_path = "/opt/*/bin/service"
				kubernetes {
					pod_labels = {
						test = "test",
					}
				}
				exports = ["metrics"]
			}
			exclude_instrument {
				exe_path = "/usr/bin/test*"
				namespace = "default"
			}
			survey {
				exe_path = "/app/microservice-*"
				name = "microservice"
				exports = ["metrics", "traces"]
			}
		}
		metrics {
			features = ["application", "network"]
			instrumentations = ["redis", "sql", "gpu", "mongo"]
			network {
				agent_ip = "0.0.0.0"
				interfaces = ["eth0"]
				source = "tc"
				protocols = ["TCP", "UDP"]
				exclude_protocols = ["ICMP"]
				sampling = 1
				cidrs = ["10.0.0.0/8"]
				cache_max_flows = 8000
				cache_active_timeout = "10s"
				direction = "ingress"
				agent_ip_iface = "local"
				agent_ip_type = "ipv4"
				exclude_interfaces = []
			}
		}
		traces {
			instrumentations = ["http", "grpc", "kafka"]
			sampler {
				name = "traceidratio"
				arg = "0.1"
			}
		}
		ebpf {
			wakeup_len = 10
			track_request_headers = true
			context_propagation = "ip"
			http_request_timeout = "10s"
			high_request_volume = true
			heuristic_sql_detect = true
			bpf_debug = false
			protocol_debug_print = false
		}
		filters {
			application {
				attr = "transport"
				not_match = "UDP"
			}
			network {
				attr = "dst_port"
				match = "53"
			}
		}
		trace_printer = "json"
		enforce_sys_caps = true
		output { /* no-op */ }
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(in), &args))

	// Verify routes
	require.Equal(t, "wildcard", args.Routes.Unmatch)
	require.Equal(t, []string{"/api/v1/*"}, args.Routes.Patterns)
	require.Equal(t, []string{"/api/v1/health"}, args.Routes.IgnorePatterns)
	require.Equal(t, "all", args.Routes.IgnoredEvents)
	require.Equal(t, "*", args.Routes.WildcardChar)

	// Verify kubernetes attributes
	require.Equal(t, "true", args.Attributes.Kubernetes.Enable)
	require.Equal(t, 15*time.Second, args.Attributes.Kubernetes.InformersSyncTimeout)
	require.Equal(t, 30*time.Minute, args.Attributes.Kubernetes.InformersResyncPeriod)
	require.Equal(t, "test", args.Attributes.Kubernetes.ClusterName)
	require.Equal(t, []string{"node"}, args.Attributes.Kubernetes.DisableInformers)
	require.True(t, args.Attributes.Kubernetes.MetaRestrictLocalNode)
	require.Equal(t, "localhost:9090", args.Attributes.Kubernetes.MetaCacheAddress)

	// Verify select attributes
	require.Len(t, args.Attributes.Select, 1)
	require.Equal(t, "sql_client_duration", args.Attributes.Select[0].Section)
	require.Equal(t, []string{"*"}, args.Attributes.Select[0].Include)
	require.Equal(t, []string{"db_statement"}, args.Attributes.Select[0].Exclude)

	// Verify network
	require.Equal(t, "0.0.0.0", args.Metrics.Network.AgentIP)
	require.Equal(t, []string{"eth0"}, args.Metrics.Network.Interfaces)
	require.Equal(t, []string{"TCP", "UDP"}, args.Metrics.Network.Protocols)
	require.Equal(t, []string{"ICMP"}, args.Metrics.Network.ExcludeProtocols)
	require.Equal(t, 1, args.Metrics.Network.Sampling)
	require.Equal(t, "10.0.0.0/8", args.Metrics.Network.CIDRs[0])
	require.Equal(t, 8000, args.Metrics.Network.CacheMaxFlows)
	require.Equal(t, 10*time.Second, args.Metrics.Network.CacheActiveTimeout)
	require.Equal(t, "ingress", args.Metrics.Network.Direction)
	require.Equal(t, "local", args.Metrics.Network.AgentIPIface)
	require.Equal(t, "ipv4", args.Metrics.Network.AgentIPType)
	require.Empty(t, args.Metrics.Network.ExcludeInterfaces)

	// Verify discovery
	require.Len(t, args.Discovery.Instrument, 2)
	require.Equal(t, "test", args.Discovery.Instrument[0].Name)
	require.Equal(t, "default", args.Discovery.Instrument[0].Namespace)
	require.Equal(t, "80,443", args.Discovery.Instrument[0].OpenPorts)
	require.Equal(t, "/usr/bin/app*", args.Discovery.Instrument[0].Path)
	require.Equal(t, []string{"metrics", "traces"}, args.Discovery.Instrument[0].ExportModes)
	require.Equal(t, "traceidratio", args.Discovery.Instrument[0].Sampler.Name)
	require.Equal(t, "0.5", args.Discovery.Instrument[0].Sampler.Arg)

	require.Len(t, args.Discovery.ExcludeInstrument, 1)
	require.Equal(t, "/usr/bin/test*", args.Discovery.ExcludeInstrument[0].Path)
	require.Equal(t, "default", args.Discovery.ExcludeInstrument[0].Namespace)

	require.Len(t, args.Discovery.Survey, 1)
	require.Equal(t, "/app/microservice-*", args.Discovery.Survey[0].Path)
	require.Equal(t, "microservice", args.Discovery.Survey[0].Name)

	// Verify metrics
	require.Equal(t, []string{"application", "network"}, args.Metrics.Features)
	require.Equal(t, []string{"redis", "sql", "gpu", "mongo"}, args.Metrics.Instrumentations)

	// Verify eBPF
	require.True(t, args.EnforceSysCaps)
	require.Equal(t, 10, args.EBPF.WakeupLen)
	require.True(t, args.EBPF.TrackRequestHeaders)
	require.Equal(t, "ip", args.EBPF.ContextPropagation)
	require.Equal(t, 10*time.Second, args.EBPF.HTTPRequestTimeout)
	require.True(t, args.EBPF.HighRequestVolume)
	require.True(t, args.EBPF.HeuristicSQLDetect)
	require.False(t, args.EBPF.BpfDebug)
	require.False(t, args.EBPF.ProtocolDebug)

	// Verify filters
	require.Len(t, args.Filters.Application, 1)
	require.Len(t, args.Filters.Network, 1)
	require.Equal(t, "transport", args.Filters.Application[0].Attr)
	require.Equal(t, "UDP", args.Filters.Application[0].NotMatch)
	require.Equal(t, "dst_port", args.Filters.Network[0].Attr)
	require.Equal(t, "53", args.Filters.Network[0].Match)

	// Verify trace printer
	require.Equal(t, "json", args.TracePrinter)

	// Verify traces
	require.Equal(t, []string{"http", "grpc", "kafka"}, args.Traces.Instrumentations)
	require.Equal(t, "traceidratio", args.Traces.Sampler.Name)
	require.Equal(t, "0.1", args.Traces.Sampler.Arg)

	// Verify validation passes
	require.NoError(t, args.Discovery.Instrument.Validate())
	require.NoError(t, args.Discovery.Survey.Validate())
}

func TestYAMLGeneration(t *testing.T) {
	opts := component.Options{
		Logger: log.NewNopLogger(),
	}

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
		Traces: Traces{
			Instrumentations: []string{"*"},
		},
		EBPF: EBPF{
			ContextPropagation: "disabled",
		},
	}

	comp := &Component{
		opts:           opts,
		args:           args,
		subprocessPort: 12345,
	}

	configPath, cleanup, err := comp.writeConfigFile()
	require.NoError(t, err)
	defer cleanup()

	// Read generated YAML
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Parse YAML
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err)

	// Verify prometheus_export
	prometheus, ok := config["prometheus_export"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, 12345, prometheus["port"])
	require.Equal(t, []interface{}{"application"}, prometheus["features"])
	require.Equal(t, []interface{}{"*"}, prometheus["instrumentations"])

	// Verify discovery
	discovery, ok := config["discovery"].(map[string]interface{})
	require.True(t, ok)
	survey, ok := discovery["survey"].([]interface{})
	require.True(t, ok)
	require.Len(t, survey, 1)
	surveyItem := survey[0].(map[string]interface{})
	require.Equal(t, ".*testserver.*", surveyItem["exe_path"])

	// Verify ebpf
	ebpf, ok := config["ebpf"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "disabled", ebpf["context_propagation"])

	// Verify traces
	traces, ok := config["traces"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, []interface{}{"*"}, traces["instrumentations"])
}

func TestYAMLGeneration_NewSchemaFields(t *testing.T) {
	opts := component.Options{Logger: log.NewNopLogger()}

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

	comp := &Component{opts: opts, args: args, subprocessPort: 9090}
	configPath, cleanup, err := comp.writeConfigFile()
	require.NoError(t, err)
	defer cleanup()

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &config))

	// Verify newly generated EBPF fields round-trip correctly.
	ebpf := config["ebpf"].(map[string]interface{})
	require.Equal(t, 64, ebpf["batch_length"])
	require.Equal(t, "5s", ebpf["batch_timeout"])
	require.Equal(t, 128, ebpf["couchbase_db_cache_size"])

	bufSizes := ebpf["buffer_sizes"].(map[string]interface{})
	require.Equal(t, 1024, bufSizes["http"])

	// Verify inject_wrapper: openai/anthropic/gemini/bedrock nested under genai.
	http := ebpf["payload_extraction"].(map[string]interface{})["http"].(map[string]interface{})
	genai := http["genai"].(map[string]interface{})
	require.Equal(t, true, genai["gemini"].(map[string]interface{})["enabled"])
	// Direct http field (not wrapped).
	require.Equal(t, true, http["graphql"].(map[string]interface{})["enabled"])

	// Verify new Stats fields.
	stats := config["stats"].(map[string]interface{})
	reverseDNS := stats["reverse_dns"].(map[string]interface{})
	require.Equal(t, 512, reverseDNS["cache_len"])
	require.Equal(t, "local", reverseDNS["type"])

	// Verify new Discovery fields.
	disc := config["discovery"].(map[string]interface{})
	require.Equal(t, true, disc["bpf_pid_filter_off"])
	require.Equal(t, []interface{}{"/usr/lib"}, disc["excluded_linux_system_paths"])
	require.Equal(t, "30s", disc["min_process_age"])
}

func TestYAMLGeneration_NetworkFlows(t *testing.T) {
	opts := component.Options{
		Logger: log.NewNopLogger(),
	}

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
			},
		},
	}

	comp := &Component{
		opts:           opts,
		args:           args,
		subprocessPort: 12345,
	}

	configPath, cleanup, err := comp.writeConfigFile()
	require.NoError(t, err)
	defer cleanup()

	// Read and parse YAML
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	require.NoError(t, err)

	// Verify network
	networkFlows, ok := config["network"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, networkFlows["enable"])
	require.Equal(t, "0.0.0.0", networkFlows["agent_ip"])
	require.Equal(t, []interface{}{"eth0"}, networkFlows["interfaces"])
	require.Equal(t, []interface{}{"TCP", "UDP"}, networkFlows["protocols"])
	require.Equal(t, 1, networkFlows["sampling"])
	require.Equal(t, []interface{}{"10.0.0.0/8"}, networkFlows["cidrs"])
	require.Equal(t, "ingress", networkFlows["direction"])
	require.Equal(t, "ipv4", networkFlows["agent_ip_type"])
}

func TestServices_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    Services
		wantErr string
	}{
		{
			name: "valid service with open_ports",
			args: Services{
				{
					OpenPorts: "80",
				},
			},
		},
		{
			name: "valid service with exe_path",
			args: Services{
				{
					Path: "/usr/bin/app",
				},
			},
		},
		{
			name: "valid service with kubernetes config",
			args: Services{
				{
					Kubernetes: KubernetesService{
						Namespace: "default",
					},
				},
			},
		},
		{
			name: "valid service with kubernetes pod labels",
			args: Services{
				{
					Kubernetes: KubernetesService{
						PodLabels: map[string]string{"app": "myapp"},
					},
				},
			},
		},
		{
			name: "invalid service - no criteria",
			args: Services{
				{
					Name: "test",
				},
			},
			wantErr: "discovery.services[0] must define at least one of: open_ports, exe_path, or kubernetes configuration",
		},
		{
			name: "multiple valid services",
			args: Services{
				{
					OpenPorts: "80",
				},
				{
					Path: "/usr/bin/app",
				},
				{
					Kubernetes: KubernetesService{
						Namespace: "default",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMetrics_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    Metrics
		wantErr string
	}{
		{
			name: "valid empty metrics",
			args: Metrics{},
		},
		{
			name: "valid instrumentations",
			args: Metrics{
				Instrumentations: []string{"http", "grpc", "*", "redis", "kafka", "sql", "gpu", "mongo"},
			},
		},
		{
			name: "invalid instrumentation",
			args: Metrics{
				Instrumentations: []string{"invalid"},
			},
			wantErr: `metrics.instrumentations: invalid value "invalid"`,
		},
		{
			name: "valid application features",
			args: Metrics{
				Features: []string{"application", "application_span", "application_service_graph", "application_process", "application_host"},
			},
		},
		{
			name: "valid network features",
			args: Metrics{
				Features: []string{"network", "network_inter_zone"},
			},
		},
		{
			name: "invalid feature",
			args: Metrics{
				Features: []string{"invalid"},
			},
			wantErr: `metrics.features: invalid value "invalid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{
			name: "empty arguments",
			args: Arguments{},
		},
		{
			name: "valid network-only configuration",
			args: Arguments{
				Metrics: Metrics{
					Features: []string{"network"},
				},
			},
		},
		{
			name: "application feature with empty services",
			args: Arguments{
				Metrics: Metrics{
					Features: []string{"application"},
				},
				Discovery: Discovery{
					Instrument: Services{},
					Survey:     Services{},
				},
			},
			wantErr: "discovery.services, discovery.instrument, or discovery.survey is required when application features are enabled",
		},
		{
			name: "valid application configuration",
			args: Arguments{
				Discovery: Discovery{
					Instrument: Services{
						{
							OpenPorts: "80",
						},
					},
				},
				Metrics: Metrics{
					Features: []string{"application"},
				},
			},
		},
		{
			name: "invalid service configuration with application feature",
			args: Arguments{
				Discovery: Discovery{
					Services: Services{
						{},
					},
				},
				Metrics: Metrics{
					Features: []string{"application"},
				},
			},
			wantErr: "must define at least one of: open_ports, exe_path, or kubernetes configuration",
		},
		{
			name: "invalid metrics configuration",
			args: Arguments{
				Metrics: Metrics{
					Features: []string{"invalid"},
				},
			},
			wantErr: "metrics.features: invalid value \"invalid\"",
		},
		{
			name: "valid trace printer",
			args: Arguments{
				TracePrinter: "json",
				Metrics: Metrics{
					Features: []string{"network"},
				},
			},
		},
		{
			name: "empty trace printer is valid",
			args: Arguments{
				Metrics: Metrics{
					Features: []string{"network"},
				},
			},
		},
		{
			name: "invalid trace printer",
			args: Arguments{
				TracePrinter: "invalid",
				Metrics: Metrics{
					Features: []string{"network"},
				},
			},
			wantErr: `trace_printer: invalid value "invalid". Valid values are: disabled, counter, text, json, json_indent`,
		},
		{
			name: "valid tracing-only configuration with trace_printer",
			args: Arguments{
				TracePrinter: "json",
			},
		},
		{
			name: "valid tracing-only configuration with output section",
			args: Arguments{
				Output: &otelcol.ConsumerArguments{
					Traces: []otelcol.Consumer{},
				},
			},
		},
		{
			name: "invalid global sampler configuration",
			args: Arguments{
				Traces: Traces{
					Sampler: SamplerConfig{
						Name: "invalid_sampler",
					},
				},
			},
			wantErr: "invalid global sampler configuration: invalid sampler name",
		},
		{
			name: "invalid service sampler configuration",
			args: Arguments{
				Discovery: Discovery{
					Services: Services{
						{
							OpenPorts: "80",
							Sampler: SamplerConfig{
								Name: "traceidratio",
								// Missing required Arg
							},
						},
					},
				},
				Metrics: Metrics{
					Features: []string{"application"},
				},
			},
			wantErr: "invalid sampler configuration in discovery.services[0]: sampler \"traceidratio\" requires an arg parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSamplerConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      SamplerConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty config is valid",
			config:      SamplerConfig{},
			expectError: false,
		},
		{
			name: "valid always_on",
			config: SamplerConfig{
				Name: "always_on",
			},
			expectError: false,
		},
		{
			name: "valid always_off",
			config: SamplerConfig{
				Name: "always_off",
			},
			expectError: false,
		},
		{
			name: "valid traceidratio with arg",
			config: SamplerConfig{
				Name: "traceidratio",
				Arg:  "0.1",
			},
			expectError: false,
		},
		{
			name: "valid parentbased_always_on",
			config: SamplerConfig{
				Name: "parentbased_always_on",
			},
			expectError: false,
		},
		{
			name: "valid parentbased_always_off",
			config: SamplerConfig{
				Name: "parentbased_always_off",
			},
			expectError: false,
		},
		{
			name: "valid parentbased_traceidratio with arg",
			config: SamplerConfig{
				Name: "parentbased_traceidratio",
				Arg:  "0.5",
			},
			expectError: false,
		},
		{
			name: "invalid sampler name",
			config: SamplerConfig{
				Name: "invalid_sampler",
			},
			expectError: true,
			errorMsg:    "invalid sampler name",
		},
		{
			name: "traceidratio without arg",
			config: SamplerConfig{
				Name: "traceidratio",
			},
			expectError: true,
			errorMsg:    "requires an arg parameter",
		},
		{
			name: "parentbased_traceidratio without arg",
			config: SamplerConfig{
				Name: "parentbased_traceidratio",
			},
			expectError: true,
			errorMsg:    "requires an arg parameter",
		},
		{
			name: "traceidratio with invalid arg",
			config: SamplerConfig{
				Name: "traceidratio",
				Arg:  "invalid",
			},
			expectError: true,
			errorMsg:    "must be a valid decimal number",
		},
		{
			name: "traceidratio with negative ratio",
			config: SamplerConfig{
				Name: "traceidratio",
				Arg:  "-0.1",
			},
			expectError: true,
			errorMsg:    "ratio must be between 0 and 1",
		},
		{
			name: "traceidratio with ratio > 1",
			config: SamplerConfig{
				Name: "traceidratio",
				Arg:  "1.5",
			},
			expectError: true,
			errorMsg:    "ratio must be between 0 and 1",
		},
		{
			name: "parentbased_traceidratio with invalid arg",
			config: SamplerConfig{
				Name: "parentbased_traceidratio",
				Arg:  "not_a_number",
			},
			expectError: true,
			errorMsg:    "must be a valid decimal number",
		},
		{
			name: "traceidratio with boundary values - 0",
			config: SamplerConfig{
				Name: "traceidratio",
				Arg:  "0",
			},
			expectError: false,
		},
		{
			name: "traceidratio with boundary values - 1",
			config: SamplerConfig{
				Name: "traceidratio",
				Arg:  "1",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTraces_Validate(t *testing.T) {
	tests := []struct {
		name    string
		args    Traces
		wantErr string
	}{
		{
			name: "valid empty config",
			args: Traces{},
		},
		{
			name: "valid instrumentations",
			args: Traces{
				Instrumentations: []string{"http", "grpc", "*", "redis", "kafka", "sql", "gpu", "mongo"},
			},
		},
		{
			name: "invalid instrumentation",
			args: Traces{
				Instrumentations: []string{"invalid"},
			},
			wantErr: `traces.instrumentations: invalid value "invalid"`,
		},
		{
			name: "valid sampler config",
			args: Traces{
				Sampler: SamplerConfig{
					Name: "traceidratio",
					Arg:  "0.5",
				},
			},
		},
		{
			name: "invalid sampler config",
			args: Traces{
				Sampler: SamplerConfig{
					Name: "invalid_sampler",
				},
			},
			wantErr: "invalid global sampler configuration",
		},
		{
			name: "sampler with invalid arg",
			args: Traces{
				Sampler: SamplerConfig{
					Name: "traceidratio",
					Arg:  "invalid",
				},
			},
			wantErr: "invalid global sampler configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestArguments_Validate_TracesOutputRequired(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{
			name: "traces with instrumentations but no output",
			args: Arguments{
				Traces: Traces{
					Instrumentations: []string{"http"},
				},
			},
			wantErr: "traces block is defined but output section is missing",
		},
		{
			name: "traces with sampler but no output",
			args: Arguments{
				Traces: Traces{
					Sampler: SamplerConfig{
						Name: "always_on",
					},
				},
			},
			wantErr: "traces block is defined but output section is missing",
		},
		{
			name: "traces with output is valid",
			args: Arguments{
				Traces: Traces{
					Instrumentations: []string{"http"},
				},
				Output: &otelcol.ConsumerArguments{},
			},
		},
		{
			name: "empty traces block is valid without output",
			args: Arguments{
				Traces: Traces{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDeprecatedFields(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	// Create a synchronized logger that protects both writing and reading
	syncLogger := log.LoggerFunc(func(keyvals ...interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		return log.NewLogfmtLogger(&buf).Log(keyvals...)
	})

	logger := level.NewFilter(syncLogger, level.AllowAll())

	comp := &Component{
		opts: component.Options{
			Logger: logger,
		},
		args: Arguments{
			Port:           "8080",
			ExecutableName: "test-app",
			Metrics: Metrics{
				Features: []string{"network"},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start component which should trigger warnings
	go comp.Run(ctx)

	// Verify warnings were logged
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		output := buf.String()
		return strings.Contains(output, "level=warn") &&
			strings.Contains(output, "open_port' field is deprecated") &&
			strings.Contains(output, "executable_name' field is deprecated")
	}, time.Second, time.Millisecond*10)
}
