//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
	require.Equal(t, "/app/microservice-*", args.Discovery.Survey[0].Service.Path)
	require.Equal(t, "microservice", args.Discovery.Survey[0].Service.Name)

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
			name: "valid service with cmd_args",
			args: Services{
				{
					CmdArgs: "--serve*",
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
			wantErr: "discovery.services[0] must define at least one of: open_ports, exe_path, cmd_args, or kubernetes configuration",
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
			name: "valid instrumentation",
			args: Metrics{Instrumentations: []string{"http"}},
		},
		{
			name:    "invalid instrumentation",
			args:    Metrics{Instrumentations: []string{"invalid"}},
			wantErr: `metrics.instrumentations: invalid value "invalid"`,
		},
		{
			name: "valid feature",
			args: Metrics{Features: []string{"application"}},
		},
		{
			name: "RED metrics without sizes",
			args: Metrics{Features: []string{"application_red"}},
		},
		{
			name: "RED and size metrics",
			args: Metrics{Features: []string{"application_red", "application_sizes"}},
		},
		{
			name: "application includes RED and sizes",
			args: Metrics{Features: []string{"application", "application_sizes"}},
		},
		{
			name: "wildcard includes RED",
			args: Metrics{Features: []string{"*", "application_sizes"}},
		},
		{
			name: "all includes RED",
			args: Metrics{Features: []string{"all", "application_sizes"}},
		},
		{
			name: "successful connections",
			args: Metrics{Features: []string{"stats_tcp_successful_connections"}},
		},
		{
			name:    "sizes require RED",
			args:    Metrics{Features: []string{"application_sizes", "application_span_otel"}},
			wantErr: "application_sizes requires application or application_red",
		},
		{
			name:    "invalid feature",
			args:    Metrics{Features: []string{"invalid"}},
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

func TestMetrics_FeatureWildcards(t *testing.T) {
	for _, f := range []string{"*", "all"} {
		m := Metrics{Features: []string{f}}
		require.True(t, m.hasAppFeature(), "%q should enable application features", f)
		require.True(t, m.hasNetworkFeature(), "%q should enable network features", f)
	}
	require.True(t, Metrics{Features: []string{"application"}}.hasAppFeature())
	require.False(t, Metrics{Features: []string{"application"}}.hasNetworkFeature())
	require.True(t, Metrics{Features: []string{"network"}}.hasNetworkFeature())
	require.False(t, Metrics{Features: []string{"network"}}.hasAppFeature())
	for _, feature := range []string{"application_red", "application_sizes"} {
		require.True(t, Metrics{Features: []string{feature}}.hasAppFeature())
		require.False(t, Metrics{Features: []string{feature}}.hasNetworkFeature())
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
					Survey:     SurveyServices{},
				},
			},
			wantErr: "discovery.services, discovery.instrument, discovery.survey, or discovery.instrument_all_local is required when application features are enabled",
		},
		{
			name:    "RED feature requires discovery",
			args:    Arguments{Metrics: Metrics{Features: []string{"application_red"}}},
			wantErr: "discovery.services, discovery.instrument, discovery.survey, or discovery.instrument_all_local is required when application features are enabled",
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
			name: "wildcard '*' feature requires discovery",
			args: Arguments{
				Metrics: Metrics{Features: []string{"*"}},
			},
			wantErr: "discovery.services, discovery.instrument, discovery.survey, or discovery.instrument_all_local is required when application features are enabled",
		},
		{
			name: "'all' feature requires discovery",
			args: Arguments{
				Metrics: Metrics{Features: []string{"all"}},
			},
			wantErr: "discovery.services, discovery.instrument, discovery.survey, or discovery.instrument_all_local is required when application features are enabled",
		},
		{
			name: "cmd_args-only instrument with application feature",
			args: Arguments{
				Discovery: Discovery{
					Instrument: Services{
						{CmdArgs: "--serve*"},
					},
				},
				Metrics: Metrics{Features: []string{"application"}},
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
			wantErr: "must define at least one of: open_ports, exe_path, cmd_args, or kubernetes configuration",
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
			wantErr: `trace_printer: invalid value "invalid". Valid values are: counter, disabled, json, json_indent, text`,
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
			name:   "empty config is valid",
			config: SamplerConfig{},
		},
		{
			name:   "valid non-ratio sampler",
			config: SamplerConfig{Name: "always_on"},
		},
		{
			name:   "valid traceidratio with arg",
			config: SamplerConfig{Name: "traceidratio", Arg: "0.1"},
		},
		{
			name:        "invalid sampler name",
			config:      SamplerConfig{Name: "invalid_sampler"},
			expectError: true,
			errorMsg:    "invalid sampler name",
		},
		{
			name:        "traceidratio without arg",
			config:      SamplerConfig{Name: "traceidratio"},
			expectError: true,
			errorMsg:    "requires an arg parameter",
		},
		{
			name:        "parentbased_traceidratio is treated as a ratio sampler",
			config:      SamplerConfig{Name: "parentbased_traceidratio"},
			expectError: true,
			errorMsg:    "requires an arg parameter",
		},
		{
			name:        "traceidratio with non-numeric arg",
			config:      SamplerConfig{Name: "traceidratio", Arg: "invalid"},
			expectError: true,
			errorMsg:    "must be a valid decimal number",
		},
		{
			name:        "traceidratio with negative ratio",
			config:      SamplerConfig{Name: "traceidratio", Arg: "-0.1"},
			expectError: true,
			errorMsg:    "ratio must be between 0 and 1",
		},
		{
			name:        "traceidratio with ratio > 1",
			config:      SamplerConfig{Name: "traceidratio", Arg: "1.5"},
			expectError: true,
			errorMsg:    "ratio must be between 0 and 1",
		},
		{
			name:   "traceidratio boundary 0",
			config: SamplerConfig{Name: "traceidratio", Arg: "0"},
		},
		{
			name:   "traceidratio boundary 1",
			config: SamplerConfig{Name: "traceidratio", Arg: "1"},
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
			name: "valid instrumentation",
			args: Traces{Instrumentations: []string{"http"}},
		},
		{
			name:    "invalid instrumentation",
			args:    Traces{Instrumentations: []string{"invalid"}},
			wantErr: `traces.instrumentations: invalid value "invalid"`,
		},
		{
			name: "valid sampler config",
			args: Traces{Sampler: SamplerConfig{Name: "traceidratio", Arg: "0.5"}},
		},
		{
			name:    "invalid sampler delegates to sampler validation",
			args:    Traces{Sampler: SamplerConfig{Name: "invalid_sampler"}},
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

func TestArguments_Validate_Beyla338Options(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{"defaults", Arguments{}, ""},
		{"resolver aliases", Arguments{NameResolver: NameResolver{Sources: []string{"dns", "ecs", "k8s", "kube", "kubernetes", "rdns"}}}, ""},
		{"unknown resolver", Arguments{NameResolver: NameResolver{Sources: []string{"invalid"}}}, "name_resolver.sources"},
		{"negative cache length", Arguments{NameResolver: NameResolver{CacheLen: -1}}, "name_resolver.cache_len"},
		{"negative cache expiry", Arguments{NameResolver: NameResolver{CacheExpiry: -time.Second}}, "name_resolver.cache_expiry"},
		{"negative ECS interval", Arguments{NameResolver: NameResolver{ECS: ECSNameResolver{RefreshInterval: -time.Second}}}, "name_resolver.ecs.refresh_interval"},
		{"negative .NET interval", Arguments{DotnetRuntimeMetrics: DotnetRuntimeMetrics{SamplingInterval: -time.Second}}, "dotnet_runtime_metrics.sampling_interval"},
		{"negative .NET timeout", Arguments{DotnetRuntimeMetrics: DotnetRuntimeMetrics{Timeout: -time.Second}}, "dotnet_runtime_metrics.timeout"},
		{"negative Kafka cache", Arguments{EBPF: EBPF{KafkaConsumerGroupCacheSize: -1}}, "ebpf.kafka_consumer_group_cache_size"},
		{"negative Kafka TTL", Arguments{EBPF: EBPF{KafkaConsumerGroupTTL: -time.Second}}, "ebpf.kafka_consumer_group_ttl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestArguments_Validate_Beyla341Options(t *testing.T) {
	appFeatures := Metrics{Features: []string{"application"}}
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{"cloud resolvers", Arguments{NameResolver: NameResolver{Sources: []string{"azure", "gce", "route53"}}}, ""},
		{"connection info feature", Arguments{
			Metrics:   Metrics{Features: []string{"application_service_graph_connection_info"}},
			Discovery: Discovery{Instrument: Services{{OpenPorts: "80"}}},
		}, ""},
		{"connection info feature requires discovery", Arguments{
			Metrics: Metrics{Features: []string{"application_service_graph_connection_info"}},
		}, "is required when application features are enabled"},
		{"negative cloud refresh interval", Arguments{CloudMetadata: CloudMetadata{RefreshInterval: -time.Second}}, "cloud_metadata.refresh_interval"},
		{"negative Route53 refresh interval", Arguments{CloudMetadata: CloudMetadata{Route53: Route53Metadata{RefreshInterval: -time.Second}}}, "cloud_metadata.route53.refresh_interval"},
		{"blank Route53 zone", Arguments{CloudMetadata: CloudMetadata{Route53: Route53Metadata{HostedZoneIDs: []string{"Z1", "/hostedzone/ "}}}}, "cloud_metadata.route53.hosted_zone_ids[1]"},
		{"deprecated ECS interval alone", Arguments{NameResolver: NameResolver{ECS: ECSNameResolver{RefreshInterval: time.Minute}}}, ""},
		{"deprecated ECS interval with cloud interval", Arguments{
			NameResolver:  NameResolver{ECS: ECSNameResolver{RefreshInterval: time.Minute}},
			CloudMetadata: CloudMetadata{RefreshInterval: time.Minute},
		}, "set only cloud_metadata.refresh_interval"},
		{"instrument_all_local satisfies application features", Arguments{
			Metrics:   appFeatures,
			Discovery: Discovery{InstrumentAllLocal: true},
		}, ""},
		{"instrument_all_local with instrument", Arguments{
			Discovery: Discovery{InstrumentAllLocal: true, Instrument: Services{{OpenPorts: "80"}}},
		}, "discovery.instrument_all_local can't be combined"},
		{"instrument_all_local with services", Arguments{
			Discovery: Discovery{InstrumentAllLocal: true, Services: Services{{OpenPorts: "80"}}},
		}, "discovery.instrument_all_local can't be combined"},
		{"instrument_all_local with survey", Arguments{
			Metrics:   appFeatures,
			Discovery: Discovery{InstrumentAllLocal: true, Survey: SurveyServices{{Service: Service{Path: "*"}}}},
		}, ""},
		{"survey socket_apps alone", Arguments{
			Metrics:   appFeatures,
			Discovery: Discovery{Survey: SurveyServices{{SocketApps: SocketApps{Enabled: true, NonRoot: true}}}},
		}, ""},
		{"survey non_root requires enabled", Arguments{
			Metrics:   appFeatures,
			Discovery: Discovery{Survey: SurveyServices{{Service: Service{Path: "*"}, SocketApps: SocketApps{NonRoot: true}}}},
		}, "socket_apps.non_root requires socket_apps.enabled"},
		{"survey entry without criteria", Arguments{
			Metrics:   appFeatures,
			Discovery: Discovery{Survey: SurveyServices{{Service: Service{Name: "svc"}}}},
		}, "discovery.survey[0] must define at least one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPeerMetadata_Validate(t *testing.T) {
	tls := PeerMetadataTLS{Enabled: true, CertFile: "tls.crt", KeyFile: "tls.key"}
	tests := []struct {
		name    string
		args    PeerMetadata
		wantErr string
	}{
		{"disabled settings aren't checked", PeerMetadata{Mode: "central", Port: -1}, ""},
		{"defaults", PeerMetadata{Enabled: true}, ""},
		{"peer mode", PeerMetadata{Enabled: true, Mode: "peer"}, ""},
		{"central mode", PeerMetadata{Enabled: true, Mode: "central"}, "doesn't implement the \"central\" mode"},
		{"unknown mode", PeerMetadata{Enabled: true, Mode: "mesh"}, "peer_metadata.mode: invalid value"},
		{"port too large", PeerMetadata{Enabled: true, Port: 65536}, "peer_metadata.port"},
		{"negative port", PeerMetadata{Enabled: true, Port: -1}, "peer_metadata.port"},
		{"IPv6 listen address", PeerMetadata{Enabled: true, ListenAddress: "::1"}, ""},
		{"host name listen address", PeerMetadata{Enabled: true, ListenAddress: "localhost"}, "peer_metadata.listen_address"},
		{"bad CIDR", PeerMetadata{Enabled: true, PeerCIDRs: []string{"10.0.0.0/8", "10.0.0.1"}}, "peer_metadata.peer_cidrs[1]"},
		{"negative refresh interval", PeerMetadata{Enabled: true, RefreshInterval: -time.Second}, "peer_metadata.refresh_interval"},
		{"sub-millisecond request timeout", PeerMetadata{Enabled: true, RequestTimeout: time.Microsecond}, "peer_metadata.request_timeout"},
		{"negative max peers", PeerMetadata{Enabled: true, MaxPeers: -1}, "peer_metadata.max_peers"},
		{"response above default cache", PeerMetadata{Enabled: true, MaxResponseBytes: 128 << 20}, "peer_metadata.max_cache_bytes"},
		{"cache below default response", PeerMetadata{Enabled: true, MaxCacheBytes: 1024}, "peer_metadata.max_cache_bytes"},
		{"raised limits", PeerMetadata{Enabled: true, MaxResponseBytes: 128 << 20, MaxCacheBytes: 256 << 20}, ""},
		{"token without TLS", PeerMetadata{Enabled: true, Token: "test-token"}, "peer_metadata.token requires peer_metadata.tls.enabled"},
		{"token with TLS", PeerMetadata{Enabled: true, Token: "test-token", TLS: tls}, ""},
		{"token with newline", PeerMetadata{Enabled: true, Token: "test\ntoken", TLS: tls}, "peer_metadata.token must not contain newlines"},
		{"TLS without key", PeerMetadata{Enabled: true, TLS: PeerMetadataTLS{Enabled: true, CertFile: "tls.crt"}}, "peer_metadata.tls requires cert_file and key_file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Arguments{PeerMetadata: tt.args}).Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
