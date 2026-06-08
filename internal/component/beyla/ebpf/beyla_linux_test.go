//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/beyla/v3/pkg/beyla"
	beylaSvc "github.com/grafana/beyla/v3/pkg/services"
	"github.com/grafana/beyla/v3/pkg/webhook/configmap"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/obi/pkg/appolly/services"
	"go.opentelemetry.io/obi/pkg/export"
	"go.opentelemetry.io/obi/pkg/export/attributes"
	"go.opentelemetry.io/obi/pkg/export/debug"
	"go.opentelemetry.io/obi/pkg/export/instrumentations"
	"go.opentelemetry.io/obi/pkg/filter"
	"go.opentelemetry.io/obi/pkg/kube/kubeflags"
	"go.opentelemetry.io/obi/pkg/obi"
	"go.opentelemetry.io/obi/pkg/transform"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util/syncbuffer"
	"github.com/grafana/alloy/syntax"
	obiCfg "go.opentelemetry.io/obi/pkg/config"
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
				cmd_args = "--config=*"
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
				cmd_args = "--skip-*"
				namespace = "default"
			}
			survey {
				exe_path = "/app/microservice-*"
				cmd_args = "--mode=survey*"
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
			context_propagation = "tcp"
			http_request_timeout = "10s"
			high_request_volume = true
			heuristic_sql_detect = true
			bpf_debug = false
			protocol_debug_print = false
			payload_extraction {
				http {
					genai {
						openai {
							enabled = true
						}
					}
				}
			}
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
	cfg, err := args.Convert()
	require.NoError(t, err)

	require.Equal(t, transform.UnmatchType("wildcard"), cfg.Routes.Unmatch)
	require.Equal(t, []string{"/api/v1/*"}, cfg.Routes.Patterns)
	//nolint:staticcheck // OBI does not expose a replacement API for ignored route patterns yet.
	require.Equal(t, []string{"/api/v1/health"}, cfg.Routes.IgnorePatterns)
	//nolint:staticcheck // OBI does not expose a replacement API for ignored route modes yet.
	require.Equal(t, transform.IgnoreMode("all"), cfg.Routes.IgnoredEvents)
	require.Equal(t, "*", cfg.Routes.WildcardChar)

	require.Equal(t, kubeflags.EnabledTrue, cfg.Attributes.Kubernetes.Enable)
	require.Equal(t, 15*time.Second, cfg.Attributes.Kubernetes.InformersSyncTimeout)
	require.Equal(t, 30*time.Minute, cfg.Attributes.Kubernetes.InformersResyncPeriod)
	require.Equal(t, "test", cfg.Attributes.Kubernetes.ClusterName)
	require.Equal(t, []string{"node"}, cfg.Attributes.Kubernetes.DisableInformers)
	require.True(t, cfg.Attributes.Kubernetes.MetaRestrictLocalNode)
	require.Equal(t, "localhost:9090", cfg.Attributes.Kubernetes.MetaCacheAddress)
	require.Len(t, cfg.Attributes.Select, 1)
	sel, ok := cfg.Attributes.Select["sql_client_duration"]
	require.True(t, ok)
	require.Equal(t, []string{"*"}, sel.Include)
	require.Equal(t, []string{"db_statement"}, sel.Exclude)

	require.Equal(t, "0.0.0.0", cfg.NetworkFlows.AgentIP)
	require.Equal(t, []string{"eth0"}, cfg.NetworkFlows.Interfaces)
	require.Equal(t, []string{"TCP", "UDP"}, cfg.NetworkFlows.Protocols)
	require.Equal(t, []string{"ICMP"}, cfg.NetworkFlows.ExcludeProtocols)
	require.Equal(t, 1, cfg.NetworkFlows.Sampling)
	require.Equal(t, "10.0.0.0/8", cfg.NetworkFlows.CIDRs[0].CIDR)
	require.Equal(t, 8000, cfg.NetworkFlows.CacheMaxFlows)
	require.Equal(t, 10*time.Second, cfg.NetworkFlows.CacheActiveTimeout)
	require.Equal(t, "ingress", cfg.NetworkFlows.Direction)
	require.Equal(t, "local", string(cfg.NetworkFlows.AgentIPIface))
	require.Equal(t, "ipv4", cfg.NetworkFlows.AgentIPType)
	require.Empty(t, cfg.NetworkFlows.ExcludeInterfaces)

	require.Len(t, cfg.Discovery.Instrument, 2)
	require.True(t, cfg.Discovery.Instrument[0].Path.IsSet())
	require.True(t, cfg.Discovery.Instrument[0].CmdArgs.IsSet())
	require.True(t, cfg.Discovery.Instrument[0].Metadata[services.AttrNamespace].IsSet())
	require.True(t, cfg.Discovery.Instrument[0].ExportModes.CanExportMetrics())
	require.True(t, cfg.Discovery.Instrument[0].ExportModes.CanExportTraces())
	require.Equal(t, &services.SamplerConfig{Name: "traceidratio", Arg: "0.5"}, cfg.Discovery.Instrument[0].SamplerConfig)
	require.True(t, cfg.Discovery.Instrument[1].PodLabels["test"].IsSet())
	require.True(t, cfg.Discovery.Instrument[1].ExportModes.CanExportMetrics())
	require.False(t, cfg.Discovery.Instrument[1].ExportModes.CanExportTraces())

	require.Len(t, cfg.Discovery.ExcludeInstrument, 1)
	require.True(t, cfg.Discovery.ExcludeInstrument[0].Path.IsSet())
	require.True(t, cfg.Discovery.ExcludeInstrument[0].CmdArgs.IsSet())

	require.Len(t, cfg.Discovery.Survey, 1)
	require.True(t, cfg.Discovery.Survey[0].Path.IsSet())
	require.True(t, cfg.Discovery.Survey[0].CmdArgs.IsSet())
	require.True(t, cfg.Discovery.Survey[0].ExportModes.CanExportMetrics())
	require.True(t, cfg.Discovery.Survey[0].ExportModes.CanExportTraces())

	require.Equal(t, export.LoadFeatures([]string{"application", "network"}), cfg.Metrics.Features)
	require.True(t, cfg.Metrics.Features.AnyAppO11yMetric())
	require.True(t, cfg.Metrics.Features.AnyNetwork())
	require.Equal(t, stringsToInstrumentations([]string{"redis", "sql", "gpu", "mongo"}), cfg.Prometheus.Instrumentations)

	require.True(t, cfg.EnforceSysCaps)
	require.Equal(t, 10, cfg.EBPF.WakeupLen)
	require.True(t, cfg.EBPF.TrackRequestHeaders)
	require.Equal(t, cfg.EBPF.ContextPropagation, obiCfg.ContextPropagationTCP)
	require.Equal(t, 10*time.Second, cfg.EBPF.HTTPRequestTimeout)
	require.True(t, cfg.EBPF.HighRequestVolume)
	require.True(t, cfg.EBPF.HeuristicSQLDetect)
	require.False(t, cfg.EBPF.BpfDebug)
	require.False(t, cfg.EBPF.ProtocolDebug)
	require.True(t, cfg.EBPF.PayloadExtraction.HTTP.GenAI.OpenAI.Enabled)
	require.True(t, cfg.EBPF.PayloadExtraction.HTTP.GenAI.OpenAI.Enabled)
	require.Len(t, cfg.Filters.Application, 1)
	require.Len(t, cfg.Filters.Network, 1)
	require.Equal(t, filter.MatchDefinition{NotMatch: "UDP"}, cfg.Filters.Application["transport"])
	require.Equal(t, filter.MatchDefinition{Match: "53"}, cfg.Filters.Network["dst_port"])
	require.Equal(t, debug.TracePrinter("json"), cfg.TracePrinter)
	require.Equal(t, stringsToInstrumentations([]string{"http", "grpc", "kafka"}), cfg.TracesReceiver.Instrumentations)
	require.Equal(t, services.SamplerConfig{Name: "traceidratio", Arg: "0.1"}, cfg.TracesReceiver.Sampler)
	require.Len(t, cfg.TracesReceiver.Traces, 0)

	instrumentConverted, err := args.Discovery.Instrument.ConvertGlob()
	require.NoError(t, err)
	require.Len(t, instrumentConverted, 2)

	surveyConverted, err := args.Discovery.Survey.ConvertGlob()
	require.NoError(t, err)
	require.Len(t, surveyConverted, 1)

	require.NoError(t, args.Discovery.Instrument.Validate())
	require.NoError(t, args.Discovery.Survey.Validate())

	require.True(t, len(cfg.Discovery.Instrument) > 0 || len(cfg.Discovery.Survey) > 0)
}

func TestArguments_TracePrinterDebug(t *testing.T) {
	test := func(debugEnabled bool, printer string, expected string) {
		const format = `
		debug = %t
		discovery {
			services {
				open_ports = "80,443"
			}
		}
		metrics {
			features = ["application", "network"]
		}
		trace_printer = "%s"
		output { /* no-op */ }
		`

		in := fmt.Sprintf(format, debugEnabled, printer)

		var args Arguments

		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		cfg, err := args.Convert()

		require.NoError(t, err)

		require.Equal(t, debug.TracePrinter(expected), cfg.TracePrinter)
	}

	// when debug is enabled, the printer will always be overridden to "text"
	// regardless of what is specified
	test(true, "json", "text")
	test(true, "text", "text")

	test(false, "text", "text")
	test(false, "json", "json")
}

func TestArguments_ConvertDefaultConfig(t *testing.T) {
	args := Arguments{}
	cfg, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, cfg.ChannelBufferLen, beyla.DefaultConfig().ChannelBufferLen)
	require.Equal(t, cfg.LogLevel, beyla.DefaultConfig().LogLevel)
	require.Equal(t, cfg.EBPF, beyla.DefaultConfig().EBPF)
	require.Equal(t, cfg.NetworkFlows, beyla.DefaultConfig().NetworkFlows)
	require.Equal(t, cfg.Grafana, beyla.DefaultConfig().Grafana)
	require.Equal(t, cfg.Attributes, beyla.DefaultConfig().Attributes)
	require.Equal(t, cfg.Routes, beyla.DefaultConfig().Routes)
	require.Equal(t, cfg.Metrics, beyla.DefaultConfig().Metrics)
	require.Equal(t, cfg.Traces, beyla.DefaultConfig().Traces)
	require.Equal(t, cfg.Prometheus, beyla.DefaultConfig().Prometheus)
	require.Equal(t, cfg.InternalMetrics, beyla.DefaultConfig().InternalMetrics)
	require.Equal(t, cfg.NetworkFlows, beyla.DefaultConfig().NetworkFlows)
	require.Equal(t, cfg.Discovery, beyla.DefaultConfig().Discovery)
	require.Equal(t, cfg.EnforceSysCaps, beyla.DefaultConfig().EnforceSysCaps)
	require.Equal(t, cfg.Stats, beyla.DefaultConfig().Stats)
}

func TestArguments_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "invalid regex",
			config: `
				discovery {
					services {
						exe_path = "["
					}
				}
				metrics {
					features = ["application"]
				}
			`,
			wantErr: "invalid regular expression \"[\": error parsing regexp: missing closing ]: `[`",
		},
		{
			name: "invalid port range",
			config: `
				discovery {
					services {
						open_ports = "-8000"
					}
				}
				metrics {
					features = ["application"]
				}
			`,
			wantErr: `invalid int enum "-8000". Must be a comma-separated list of integers or ranges (e.g. 8000-8999)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tt.config), &args))
			_, err := args.Convert()
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestArguments_InvalidExportModes(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name: "invalid selector",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = ["foo"]
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
		{
			name: "empty selector",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = [""]
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
		{
			name: "one invalid selector",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = ["metrics", "not traces"]
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tt.config), &args))
			_, err := convertExportModes(args.Discovery.Services[0].ExportModes)
			require.Error(t, err)
		})
	}
}

func TestArguments_ValidExportModes(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name: "empty selector",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = []
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
		{
			name: "traces",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = ["traces"]
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
		{
			name: "metrics",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = ["metrics"]
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
		{
			name: "metrics and traces",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = ["metrics", "traces"]
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
		{
			name: "traces and metrics",
			config: `
				discovery {
					services {
						open_ports = "8000"
						exports = ["traces", "metrics"]
					}
				}
				metrics {
					features = ["application"]
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tt.config), &args))
		})
	}
}

func TestConvert_Routes(t *testing.T) {
	args := Routes{
		Unmatch:        "wildcard",
		Patterns:       []string{"/api/v1/*"},
		IgnorePatterns: []string{"/api/v1/health"},
		IgnoredEvents:  "all",
	}

	expectedConfig := &transform.RoutesConfig{
		Unmatch:                   transform.UnmatchType(args.Unmatch),
		Patterns:                  args.Patterns,
		IgnorePatterns:            args.IgnorePatterns,
		IgnoredEvents:             transform.IgnoreMode(args.IgnoredEvents),
		WildcardChar:              "*",
		MaxPathSegmentCardinality: 10,
	}

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_SamplerConfig(t *testing.T) {
	tests := []struct {
		name     string
		args     SamplerConfig
		expected services.SamplerConfig
	}{
		{
			name: "with name and arg",
			args: SamplerConfig{
				Name: "traceidratio",
				Arg:  "0.5",
			},
			expected: services.SamplerConfig{
				Name: "traceidratio",
				Arg:  "0.5",
			},
		},
		{
			name: "empty config",
			args: SamplerConfig{},
			expected: services.SamplerConfig{
				Name: "",
				Arg:  "",
			},
		},
		{
			name: "only name",
			args: SamplerConfig{
				Name: "always_on",
			},
			expected: services.SamplerConfig{
				Name: "always_on",
				Arg:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.args.Convert()
			require.Equal(t, tt.expected, config)
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

func TestConvert_Attributes(t *testing.T) {
	args := Attributes{
		Kubernetes: KubernetesDecorator{
			Enable:               "true",
			InformersSyncTimeout: 15 * time.Second,
			MetaCacheAddress:     "localhost:9090",
		},
		Select: Selections{
			{
				Section: "sql_client_duration",
				Include: []string{"*"},
				Exclude: []string{"db_statement"},
			},
		},
		InstanceID: InstanceIDConfig{
			OverrideHostname: "test",
		},
	}

	expectedConfig := beyla.Attributes{
		Kubernetes: transform.KubernetesDecorator{
			Enable:                   kubeflags.EnableFlag(args.Kubernetes.Enable),
			InformersSyncTimeout:     15 * time.Second,
			InformersResyncPeriod:    30 * time.Minute,
			ReconnectInitialInterval: beyla.DefaultConfig().Attributes.Kubernetes.ReconnectInitialInterval,
			ResourceLabels:           beyla.DefaultConfig().Attributes.Kubernetes.ResourceLabels,
			MetaCacheAddress:         "localhost:9090",
		},
		HostID: beyla.HostIDConfig{},
		Select: attributes.Selection{
			"sql_client_duration": {
				Include: []string{"*"},
				Exclude: []string{"db_statement"},
			},
		},
		RenameUnresolvedHosts:          "unresolved",
		RenameUnresolvedHostsOutgoing:  "outgoing",
		RenameUnresolvedHostsIncoming:  "incoming",
		MetricSpanNameAggregationLimit: 100,
	}
	expectedConfig.InstanceID.OverrideHostname = "test"
	expectedConfig.InstanceID.HostnameDNSResolution = true
	expectedConfig.MetadataRetry = beyla.DefaultConfig().Attributes.MetadataRetry
	expectedConfig.Kubernetes.ReconnectInitialInterval = beyla.DefaultConfig().Attributes.Kubernetes.ReconnectInitialInterval

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_Discovery(t *testing.T) {
	args := Discovery{
		Instrument: []Service{
			{
				Name:           "test",
				Namespace:      "default",
				OpenPorts:      "80",
				CmdArgs:        "--serve*",
				ContainersOnly: true,
				ExportModes:    []string{"metrics"},
				Sampler: SamplerConfig{
					Arg:  "0.5",
					Name: "traceidratio",
				},
			},
			{
				Kubernetes: KubernetesService{
					Namespace:      "default",
					DeploymentName: "test",
				},
			},
			{
				Kubernetes: KubernetesService{
					Namespace:       "default",
					PodName:         "test",
					DeploymentName:  "test",
					ReplicaSetName:  "test",
					StatefulSetName: "test",
					DaemonSetName:   "test",
					OwnerName:       "test",
					PodLabels:       map[string]string{"test": "test"},
					PodAnnotations:  map[string]string{"test": "test"},
				},
			},
		},
		ExcludeInstrument: []Service{
			{
				Name:      "test",
				Namespace: "default",
			},
		},
		DefaultExcludeInstrument: []Service{},
	}
	config, err := args.Convert()

	require.NoError(t, err)
	require.Len(t, config.Instrument, 3)
	require.Equal(t, services.IntEnum{Ranges: []services.IntRange{{Start: 80, End: 0}}}, config.Instrument[0].OpenPorts)
	require.True(t, config.Instrument[0].CmdArgs.IsSet())
	require.True(t, config.Instrument[0].ContainersOnly)
	require.True(t, config.Instrument[0].ExportModes.CanExportMetrics())
	require.False(t, config.Instrument[0].ExportModes.CanExportTraces())
	require.Equal(t, &services.SamplerConfig{Name: "traceidratio", Arg: "0.5"}, config.Instrument[0].SamplerConfig)
	require.True(t, config.Instrument[1].Metadata[services.AttrNamespace].IsSet())
	require.True(t, config.Instrument[1].Metadata[services.AttrDeploymentName].IsSet())
	_, exists := config.Instrument[1].Metadata[services.AttrDaemonSetName]
	require.False(t, exists)
	require.True(t, config.Instrument[2].Metadata[services.AttrNamespace].IsSet())
	require.True(t, config.Instrument[2].Metadata[services.AttrPodName].IsSet())
	require.True(t, config.Instrument[2].Metadata[services.AttrDeploymentName].IsSet())
	require.True(t, config.Instrument[2].Metadata[services.AttrReplicaSetName].IsSet())
	require.True(t, config.Instrument[2].Metadata[services.AttrStatefulSetName].IsSet())
	require.True(t, config.Instrument[2].Metadata[services.AttrDaemonSetName].IsSet())
	require.True(t, config.Instrument[2].Metadata[services.AttrOwnerName].IsSet())
	require.True(t, config.Instrument[2].PodLabels["test"].IsSet())
	require.True(t, config.Instrument[2].PodAnnotations["test"].IsSet())
	require.NoError(t, config.Instrument.Validate())
	require.Len(t, config.ExcludeInstrument, 1)
	require.Equal(t, true, config.ExcludeOTelInstrumentedServices)
}

func TestConvert_Prometheus(t *testing.T) {
	args := Metrics{
		Features:                        []string{"application", "network"},
		Instrumentations:                []string{"redis", "sql"},
		AllowServiceGraphSelfReferences: true,
		ExtraResourceLabels:             nil,
		ExtraSpanResourceLabels:         []string{"service.version"},
	}

	expectedConfig := beyla.DefaultConfig().Prometheus
	expectedConfig.Instrumentations = stringsToInstrumentations(args.Instrumentations)
	expectedConfig.AllowServiceGraphSelfReferences = true
	expectedConfig.ExtraSpanResourceLabels = args.ExtraSpanResourceLabels

	config := args.Convert()

	require.Equal(t, expectedConfig, config)

	args = Metrics{
		Features:                        []string{"application", "network"},
		Instrumentations:                []string{"redis", "sql"},
		AllowServiceGraphSelfReferences: true,
		ExtraResourceLabels:             []string{"service.version"},
	}

	expectedConfig = beyla.DefaultConfig().Prometheus
	expectedConfig.Instrumentations = stringsToInstrumentations(args.Instrumentations)
	expectedConfig.AllowServiceGraphSelfReferences = true
	expectedConfig.ExtraResourceLabels = args.ExtraResourceLabels

	config = args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_Network(t *testing.T) {
	args := Network{
		AgentIP:          "0.0.0.0",
		Interfaces:       []string{"eth0"},
		Protocols:        []string{"TCP", "UDP"},
		ExcludeProtocols: []string{"ICMP"},
		Sampling:         1,
		CIDRs:            []string{"10.0.0.0/8"},
	}

	expectedConfig := beyla.DefaultConfig().NetworkFlows
	expectedConfig.AgentIP = "0.0.0.0"
	expectedConfig.Interfaces = args.Interfaces
	expectedConfig.Protocols = args.Protocols
	expectedConfig.ExcludeProtocols = args.ExcludeProtocols
	expectedConfig.Sampling = 1
	expectedConfig.Print = false
	if len(args.CIDRs) > 0 {
		_ = expectedConfig.CIDRs.UnmarshalText([]byte(strings.Join(args.CIDRs, ",")))
	}

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_Stats(t *testing.T) {
	args := Stats{
		AgentIP:      "0.0.0.0",
		AgentIPIface: "local",
		AgentIPType:  "ipv4",
		CIDRs:        []string{"10.0.0.0/8"},
		Print:        true,
	}

	expectedConfig := beyla.DefaultConfig().Stats
	expectedConfig.AgentIP = args.AgentIP
	expectedConfig.AgentIPIface = obi.AgentTypeIface(args.AgentIPIface)
	expectedConfig.AgentIPType = args.AgentIPType
	if len(args.CIDRs) > 0 {
		_ = expectedConfig.CIDRs.UnmarshalText([]byte(strings.Join(args.CIDRs, ",")))
	}
	expectedConfig.Print = args.Print

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_EBPF(t *testing.T) {
	args := EBPF{
		WakeupLen:           10,
		TrackRequestHeaders: true,
		HighRequestVolume:   true,
		HeuristicSQLDetect:  true,
		ContextPropagation:  "tcp",
		BpfDebug:            true,
		ProtocolDebug:       true,
		PayloadExtraction: PayloadExtraction{
			HTTP: HTTPPayloadExtraction{
				GenAI: GenAI{
					OpenAI: ProtocolToggle{Enabled: true},
				},
			},
		},
	}

	expectedConfig := beyla.DefaultConfig().EBPF
	expectedConfig.WakeupLen = 10
	expectedConfig.TrackRequestHeaders = true
	expectedConfig.HighRequestVolume = true
	expectedConfig.HeuristicSQLDetect = true
	expectedConfig.ContextPropagation = obiCfg.ContextPropagationTCP
	expectedConfig.BpfDebug = true
	expectedConfig.ProtocolDebug = true
	expectedConfig.PayloadExtraction.HTTP.GenAI.OpenAI.Enabled = true

	config, err := args.Convert()
	require.NoError(t, err)

	require.Equal(t, expectedConfig, *config)
}

func TestConvert_EBPF_ContextPropagationIPCompatibility(t *testing.T) {
	args := EBPF{
		ContextPropagation: "ip",
	}

	expectedConfig := beyla.DefaultConfig().EBPF

	config, err := args.Convert()
	require.NoError(t, err)

	require.Equal(t, expectedConfig, *config)
}

func TestConvert_Filters(t *testing.T) {
	args := Filters{
		Application: AttributeFamilies{
			{
				Attr:     "transport",
				NotMatch: "UDP",
			},
		},
		Network: AttributeFamilies{
			{
				Attr:  "dst_port",
				Match: "53",
			},
		},
	}
	expectedConfig := filter.AttributesConfig{
		Application: filter.AttributeFamilyConfig{
			"transport": filter.MatchDefinition{
				NotMatch: "UDP",
			},
		},
		Network: filter.AttributeFamilyConfig{
			"dst_port": filter.MatchDefinition{
				Match: "53",
			},
		},
	}
	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_Filters_NumericOps(t *testing.T) {
	gt, eq := 200, 404
	args := Filters{
		Application: AttributeFamilies{
			{Attr: "http_response_status_code", GreaterThan: &gt, Equals: &eq},
		},
		Network: AttributeFamilies{
			{Attr: "dst_port", LessThan: intptr(8080)},
		},
	}
	cfg := args.Convert()
	require.Equal(t, &gt, cfg.Application["http_response_status_code"].GreaterThan)
	require.Equal(t, &eq, cfg.Application["http_response_status_code"].Equals)
	require.Equal(t, 8080, *cfg.Network["dst_port"].LessThan)
}

func intptr(i int) *int { return &i }

func TestConvert_InjectorWebhook(t *testing.T) {
	args := InjectorWebhook{
		ExternalWebhook: "beyla-controller",
	}

	expectedConfig := beyla.DefaultConfig().Injector.Webhook
	expectedConfig.ExternalWebhook = "beyla-controller"

	config := args.Convert()
	require.Equal(t, expectedConfig, config)
}

func TestConvert_InjectorSDKExport(t *testing.T) {
	traces := true
	metrics := false
	logs := true
	args := InjectorSDKExport{
		Traces:  &traces,
		Metrics: &metrics,
		Logs:    &logs,
	}

	expectedConfig := beyla.DefaultConfig().Injector.ExportedSignals
	expectedConfig.Traces = &traces
	expectedConfig.Metrics = &metrics
	expectedConfig.Logs = &logs

	config := args.Convert()
	require.Equal(t, expectedConfig, config)
}

func TestConvert_InjectorSDKResource(t *testing.T) {
	addK8s := true
	addK8sIP := true
	useLabels := false
	args := InjectorSDKResource{
		Attributes:                     map[string]string{"environment": "dev"},
		AddK8sUIDAttributes:            &addK8s,
		AddK8sIPAttribute:              &addK8sIP,
		UseLabelsForResourceAttributes: &useLabels,
	}

	expectedConfig := beyla.DefaultConfig().Injector.Resources
	expectedConfig.Attributes = map[string]string{"environment": "dev"}
	expectedConfig.AddK8sUIDAttributes = true
	expectedConfig.AddK8sIPAttribute = true
	expectedConfig.UseLabelsForResourceAttributes = false

	config := args.Convert()
	require.Equal(t, expectedConfig, config)
}

func TestConvert_Injector(t *testing.T) {
	args := Injector{
		ImageVersion:   "1.0.0",
		Propagators:    []string{"tracecontext", "baggage"},
		EnabledSDKs:    []string{"java", "dotnet"},
		DefaultSampler: SamplerConfig{Name: "always_on"},
	}

	javaSDK := beylaSvc.InstrumentableType{}
	require.NoError(t, javaSDK.UnmarshalText([]byte("java")))
	dotnetSDK := beylaSvc.InstrumentableType{}
	require.NoError(t, dotnetSDK.UnmarshalText([]byte("dotnet")))

	expectedConfig := beyla.DefaultConfig().Injector
	expectedConfig.Webhook = InjectorWebhook{}.Convert()
	expectedConfig.ExportedSignals = InjectorSDKExport{}.Convert()
	expectedConfig.Resources = InjectorSDKResource{}.Convert()
	expectedConfig.ImageVersion = "1.0.0"
	expectedConfig.Propagators = []string{"tracecontext", "baggage"}
	expectedConfig.EnabledSDKs = []beylaSvc.InstrumentableType{javaSDK, dotnetSDK}
	s := SamplerConfig{Name: "always_on"}.Convert()
	expectedConfig.DefaultSampler = &s

	config, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, expectedConfig, config)
}

func TestConvert_Injector_Errors(t *testing.T) {
	tests := []struct {
		name    string
		args    Injector
		wantErr string
	}{
		{
			name: "valid injector config",
			args: Injector{
				Webhook:     InjectorWebhook{ExternalWebhook: "beyla-controller"},
				EnabledSDKs: []string{"java", "dotnet"},
			},
		},
		{
			name: "invalid enabled sdk",
			args: Injector{
				EnabledSDKs: []string{"invalid-sdk"},
			},
			wantErr: "injector.enabled_sdks:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.args.Convert()
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
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
			name: "valid service with cmd_args",
			args: Services{
				{
					CmdArgs: "--serve*",
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
			name: "valid instrumentations",
			args: Metrics{
				Instrumentations: []string{"http", "grpc", "*", "redis", "kafka", "sql", "gpu", "mongo", "memcached", "genai"},
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
			wantErr: "must define at least one of: open_ports, exe_path, cmd_args, or kubernetes configuration",
		},
		{
			name: "valid cmd_args-only configuration with application feature",
			args: Arguments{
				Discovery: Discovery{
					Instrument: Services{
						{
							CmdArgs: "--serve*",
						},
					},
				},
				Metrics: Metrics{
					Features: []string{"application"},
				},
			},
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
				// No metrics features defined
			},
		},
		{
			name: "valid tracing-only configuration with output section",
			args: Arguments{
				Output: &otelcol.ConsumerArguments{
					Traces: []otelcol.Consumer{},
				},
				// No metrics features defined
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

func TestDeprecatedFields(t *testing.T) {
	var buf syncbuffer.Buffer

	logger, err := logging.New(&buf, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	comp := &Component{
		opts: component.Options{
			SLogger: logger.Slog(),
		},
		args: Arguments{
			Port:           "8080",
			ExecutableName: "test-app",
			Metrics: Metrics{
				Features: []string{"network"},
			},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Start component which should trigger warnings
	go comp.Run(ctx)

	// Verify warnings were logged
	require.Eventually(t, func() bool {
		output := buf.String()
		return strings.Contains(output, "level=warn") &&
			strings.Contains(output, "open_port' field is deprecated") &&
			strings.Contains(output, "executable_name' field is deprecated")
	}, time.Second, time.Millisecond*10)
}

func TestTraces_Convert(t *testing.T) {
	tests := []struct {
		name      string
		args      Traces
		consumers []otelcol.Consumer
		expected  beyla.TracesReceiverConfig
	}{
		{
			name:      "empty config uses default instrumentations",
			args:      Traces{},
			consumers: nil,
			expected: beyla.TracesReceiverConfig{
				Traces: []beyla.Consumer{},
				Instrumentations: []instrumentations.Instrumentation{
					instrumentations.InstrumentationALL,
				},
			},
		},
		{
			name: "custom instrumentations",
			args: Traces{
				Instrumentations: []string{"http", "grpc"},
			},
			consumers: nil,
			expected: beyla.TracesReceiverConfig{
				Traces:           []beyla.Consumer{},
				Instrumentations: []instrumentations.Instrumentation{"http", "grpc"},
			},
		},
		{
			name: "with consumers",
			args: Traces{
				Instrumentations: []string{"kafka"},
			},
			consumers: []otelcol.Consumer{
				// Mock consumer would go here in real test
			},
			expected: beyla.TracesReceiverConfig{
				Traces:           []beyla.Consumer{},
				Instrumentations: []instrumentations.Instrumentation{"kafka"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.Convert(tt.consumers)
			require.Equal(t, tt.expected.Instrumentations, result.Instrumentations)
			require.Len(t, result.Traces, len(tt.expected.Traces))
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
				Instrumentations: []string{"http", "grpc", "*", "redis", "kafka", "sql", "gpu", "mongo", "memcached", "genai"},
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

func TestServices_Convert_SamplerConfig(t *testing.T) {
	tests := []struct {
		name                string
		services            Services
		expectSamplerConfig bool
		expectedSamplerName string
	}{
		{
			name: "service with empty sampler config",
			services: Services{
				{
					OpenPorts: "80",
					Sampler:   SamplerConfig{}, // Empty sampler
				},
			},
			expectSamplerConfig: false,
		},
		{
			name: "service with sampler config",
			services: Services{
				{
					OpenPorts: "80",
					Sampler: SamplerConfig{
						Name: "traceidratio",
						Arg:  "0.5",
					},
				},
			},
			expectSamplerConfig: true,
			expectedSamplerName: "traceidratio",
		},
		{
			name: "service with only sampler name",
			services: Services{
				{
					OpenPorts: "80",
					Sampler: SamplerConfig{
						Name: "always_on",
					},
				},
			},
			expectSamplerConfig: true,
			expectedSamplerName: "always_on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.services.Convert()
			require.NoError(t, err)
			require.Len(t, result, 1)

			if tt.expectSamplerConfig {
				require.NotNil(t, result[0].SamplerConfig)
				require.Equal(t, services.SamplerName(tt.expectedSamplerName), result[0].SamplerConfig.Name)
			} else {
				require.Nil(t, result[0].SamplerConfig)
			}
		})
	}
}

func TestEnvVars(t *testing.T) {
	comp := &Component{
		args: Arguments{
			TracePrinter: "text",
		},
	}

	t.Setenv("BEYLA_TRACE_PRINTER", "json")

	cfg, err := comp.loadConfig()

	require.NoError(t, err)
	require.Equal(t, debug.TracePrinterJSON, cfg.TracePrinter)
}

func TestMetrics_Validate_ExemplarFilter(t *testing.T) {
	tests := []struct {
		name    string
		args    Metrics
		wantErr string
	}{
		{
			name: "empty exemplar_filter is valid",
			args: Metrics{},
		},
		{
			name: "always_on is valid",
			args: Metrics{ExemplarFilter: "always_on"},
		},
		{
			name: "always_off is valid",
			args: Metrics{ExemplarFilter: "always_off"},
		},
		{
			name: "trace_based is valid",
			args: Metrics{ExemplarFilter: "trace_based"},
		},
		{
			name:    "invalid value",
			args:    Metrics{ExemplarFilter: "invalid"},
			wantErr: `metrics.exemplar_filter: invalid value "invalid"`,
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

func TestMetrics_Convert_ExemplarFilter(t *testing.T) {
	args := Metrics{ExemplarFilter: "always_on"}
	cfg := args.Convert()
	require.Equal(t, "always_on", cfg.ExemplarFilter)

	args = Metrics{ExemplarFilter: ""}
	cfg = args.Convert()
	require.Equal(t, beyla.DefaultConfig().Prometheus.ExemplarFilter, cfg.ExemplarFilter)
}

func TestMetrics_Validate_FeatureWildcard(t *testing.T) {
	tests := []struct {
		name string
		args Metrics
	}{
		{name: "wildcard star", args: Metrics{Features: []string{"*"}}},
		{name: "wildcard all", args: Metrics{Features: []string{"all"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.args.Validate())
			require.True(t, tt.args.hasAppFeature())
		})
	}
}

func TestEBPF_Convert_MapsConfig(t *testing.T) {
	args := EBPF{MapsConfig: EBPFMapsConfig{GlobalScaleFactor: 2}}
	cfg, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, 2, cfg.MapsConfig.GlobalScaleFactor)

	args = EBPF{}
	cfg, err = args.Convert()
	require.NoError(t, err)
	require.Equal(t, 0, cfg.MapsConfig.GlobalScaleFactor)
}

func TestSurveyDisabled(t *testing.T) {
	comp := &Component{
		args: Arguments{
			TracePrinter: "text",
		},
	}

	cfg, err := comp.loadConfig()

	require.NoError(t, err)
	require.False(t, cfg.Discovery.SurveyEnabled())
	require.NotEqual(t, beylaSvc.DefaultExcludeServicesWithSurvey, cfg.Discovery.DefaultExcludeServices)
	require.NotEqual(t, beylaSvc.DefaultExcludeInstrumentWithSurvey, cfg.Discovery.DefaultExcludeInstrument)
}

func TestSurveyEnabled(t *testing.T) {
	comp := &Component{
		args: Arguments{
			TracePrinter: "text",
			Discovery: Discovery{
				Survey: Services{
					{
						Name: "foo",
					},
				},
			},
		},
	}

	cfg, err := comp.loadConfig()

	require.NoError(t, err)
	require.Len(t, cfg.Discovery.Survey, 1)
	require.True(t, cfg.Discovery.SurveyEnabled())
	require.Equal(t, beylaSvc.DefaultExcludeServicesWithSurvey, cfg.Discovery.DefaultExcludeServices)
	require.Equal(t, beylaSvc.DefaultExcludeInstrumentWithSurvey, cfg.Discovery.DefaultExcludeInstrument)
}

// globPtr returns a pointer to a GlobAttr compiled from the given pattern, for
// building the pointer-valued maps in services.GlobAttributes.
func globPtr(pattern string) *services.GlobAttr {
	g := services.NewGlob(pattern)
	return &g
}

func TestSelectorFromGlob(t *testing.T) {
	tests := []struct {
		name     string
		attrs    services.GlobAttributes
		expected configmap.K8sSelector
	}{
		{
			name:     "empty attributes produce an empty selector",
			attrs:    services.GlobAttributes{},
			expected: configmap.K8sSelector{},
		},
		{
			// Port-based criteria are a process-level match and have no
			// Kubernetes selector equivalent, so they are ignored here.
			name: "open ports only produce an empty selector",
			attrs: services.GlobAttributes{
				OpenPorts: services.IntEnum{Ranges: []services.IntRange{{Start: 80}, {Start: 443}}},
			},
			expected: configmap.K8sSelector{},
		},
		{
			name: "namespace only",
			attrs: services.GlobAttributes{
				Metadata: services.MetadataGlobMap{
					services.AttrNamespace: globPtr("default"),
				},
			},
			expected: configmap.K8sSelector{
				Namespaces: []services.GlobAttr{services.NewGlob("default")},
			},
		},
		{
			name: "generic owner name sets no owner kind",
			attrs: services.GlobAttributes{
				Metadata: services.MetadataGlobMap{
					services.AttrOwnerName: globPtr("my-app"),
				},
			},
			expected: configmap.K8sSelector{
				OwnerNames: []services.GlobAttr{services.NewGlob("my-app")},
			},
		},
		{
			name: "specific kind sets both owner name and kind",
			attrs: services.GlobAttributes{
				Metadata: services.MetadataGlobMap{
					services.AttrDeploymentName: globPtr("web"),
				},
			},
			expected: configmap.K8sSelector{
				OwnerNames: []services.GlobAttr{services.NewGlob("web")},
				OwnerKinds: []string{"Deployment"},
			},
		},
		{
			name: "owner name takes precedence over a specific kind",
			attrs: services.GlobAttributes{
				Metadata: services.MetadataGlobMap{
					services.AttrOwnerName:      globPtr("my-app"),
					services.AttrDeploymentName: globPtr("web"),
				},
			},
			expected: configmap.K8sSelector{
				OwnerNames: []services.GlobAttr{services.NewGlob("my-app")},
			},
		},
		{
			name: "pod labels and annotations",
			attrs: services.GlobAttributes{
				PodLabels: map[string]*services.GlobAttr{
					"app": globPtr("frontend"),
				},
				PodAnnotations: map[string]*services.GlobAttr{
					"team": globPtr("backend"),
				},
			},
			expected: configmap.K8sSelector{
				PodLabels:      map[string]services.GlobAttr{"app": services.NewGlob("frontend")},
				PodAnnotations: map[string]services.GlobAttr{"team": services.NewGlob("backend")},
			},
		},
		{
			name: "all fields combined",
			attrs: services.GlobAttributes{
				Metadata: services.MetadataGlobMap{
					services.AttrNamespace:       globPtr("prod"),
					services.AttrStatefulSetName: globPtr("db"),
				},
				PodLabels: map[string]*services.GlobAttr{
					"tier": globPtr("data"),
				},
				PodAnnotations: map[string]*services.GlobAttr{
					"owner": globPtr("team-a"),
				},
			},
			expected: configmap.K8sSelector{
				Namespaces:     []services.GlobAttr{services.NewGlob("prod")},
				OwnerNames:     []services.GlobAttr{services.NewGlob("db")},
				OwnerKinds:     []string{"StatefulSet"},
				PodLabels:      map[string]services.GlobAttr{"tier": services.NewGlob("data")},
				PodAnnotations: map[string]services.GlobAttr{"owner": services.NewGlob("team-a")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectorFromGlob(&tt.attrs)
			require.Equal(t, tt.expected, got)
		})
	}
}

// TestSelectorFromGlob_OwnerKinds verifies that each supported k8s owner
// metadata key maps to the expected OwnerKind.
func TestSelectorFromGlob_OwnerKinds(t *testing.T) {
	cases := []struct {
		attr string
		kind string
	}{
		{services.AttrDeploymentName, "Deployment"},
		{services.AttrDaemonSetName, "DaemonSet"},
		{services.AttrReplicaSetName, "ReplicaSet"},
		{services.AttrStatefulSetName, "StatefulSet"},
		{services.AttrJobName, "Job"},
		{services.AttrCronJobName, "CronJob"},
		{services.AttrPodName, "Pod"},
	}

	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			attrs := services.GlobAttributes{
				Metadata: services.MetadataGlobMap{
					c.attr: globPtr("name-*"),
				},
			}
			got := selectorFromGlob(&attrs)
			require.Equal(t, []services.GlobAttr{services.NewGlob("name-*")}, got.OwnerNames)
			require.Equal(t, []string{c.kind}, got.OwnerKinds)
		})
	}
}

func TestSelectorsFromInstrument(t *testing.T) {
	t.Run("filters out empty selectors", func(t *testing.T) {
		g := services.GlobDefinitionCriteria{
			{
				Metadata: services.MetadataGlobMap{
					services.AttrNamespace: globPtr("default"),
				},
			},
			{}, // no selectable criteria, should be skipped
			{
				PodLabels: map[string]*services.GlobAttr{
					"app": globPtr("api"),
				},
			},
		}

		got := selectorsFromInstrument(g)
		require.Len(t, got, 2)
		require.Equal(t, []services.GlobAttr{services.NewGlob("default")}, got[0].Namespaces)
		require.Equal(t, map[string]services.GlobAttr{"app": services.NewGlob("api")}, got[1].PodLabels)
	})

	t.Run("all-empty criteria return nil", func(t *testing.T) {
		g := services.GlobDefinitionCriteria{{}, {}}
		require.Nil(t, selectorsFromInstrument(g))
	})

	t.Run("nil criteria return nil", func(t *testing.T) {
		require.Nil(t, selectorsFromInstrument(nil))
	})
}

func strptr(s string) *string { return &s }

func TestConvert_EBPF_NewScalars(t *testing.T) {
	args := EBPF{
		InstrumentCuda:        "on",
		TrafficControlBackend: "tcx",
		MaxTransactionTime:    5 * time.Second,
		DNSRequestTimeout:     time.Second,
		BufferSizes:           BufferSizes{HTTP: 1024, MySQL: 512},
	}
	cfg, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, obiCfg.CudaModeOn, cfg.InstrumentCuda)
	require.Equal(t, obiCfg.TCBackendTCX, cfg.TCBackend)
	require.Equal(t, 5*time.Second, cfg.MaxTransactionTime)
	require.Equal(t, time.Second, cfg.DNSRequestTimeout)
	require.Equal(t, uint32(1024), cfg.BufferSizes.HTTP)
	require.Equal(t, uint32(512), cfg.BufferSizes.MySQL)
}

func TestValidate_EBPF_Enums(t *testing.T) {
	bad := Arguments{EBPF: EBPF{InstrumentCuda: "bogus"}}
	require.ErrorContains(t, bad.Validate(), "ebpf.instrument_cuda")

	badTC := Arguments{EBPF: EBPF{TrafficControlBackend: "bogus"}}
	require.ErrorContains(t, badTC.Validate(), "ebpf.traffic_control_backend")

	badBuf := Arguments{EBPF: EBPF{BufferSizes: BufferSizes{HTTP: 70000}}}
	require.ErrorContains(t, badBuf.Validate(), "ebpf.buffer_sizes")
}

func TestConvert_Attributes_NewScalars(t *testing.T) {
	args := Attributes{
		Kubernetes:                    KubernetesDecorator{Enable: "true"},
		RenameUnresolvedHosts:         strptr("unknown"),
		RenameUnresolvedHostsOutgoing: strptr("out"),
		RenameUnresolvedHostsIncoming: strptr("in"),
		MetricSpanNamesLimit:          50,
		HostID:                        HostIDConfig{Override: "my-host"},
		MetadataRetry: MetadataRetry{
			Timeout:       10 * time.Second,
			StartInterval: time.Second,
			MaxInterval:   2 * time.Second,
		},
	}
	cfg := args.Convert()
	require.Equal(t, "unknown", cfg.RenameUnresolvedHosts)
	require.Equal(t, "out", cfg.RenameUnresolvedHostsOutgoing)
	require.Equal(t, "in", cfg.RenameUnresolvedHostsIncoming)
	require.Equal(t, 50, cfg.MetricSpanNameAggregationLimit)
	require.Equal(t, "my-host", cfg.HostID.Override)
	require.Equal(t, 10*time.Second, cfg.MetadataRetry.Timeout)
	require.Equal(t, time.Second, cfg.MetadataRetry.StartInterval)
	require.Equal(t, 2*time.Second, cfg.MetadataRetry.MaxInterval)
}

func TestConvert_Attributes_RenameUnresolvedHostsDisable(t *testing.T) {
	empty := ""
	args := Attributes{
		Kubernetes:            KubernetesDecorator{Enable: "true"},
		RenameUnresolvedHosts: &empty,
	}
	cfg := args.Convert()
	require.Equal(t, "", cfg.RenameUnresolvedHosts)
}

func TestConvert_Attributes_RenameUnresolvedHostsDefault(t *testing.T) {
	args := Attributes{
		Kubernetes: KubernetesDecorator{Enable: "true"},
	}
	cfg := args.Convert()
	require.Equal(t, beyla.DefaultConfig().Attributes.RenameUnresolvedHosts, cfg.RenameUnresolvedHosts)
	require.NotEmpty(t, cfg.RenameUnresolvedHosts)
}

func TestConvert_Attributes_Kubernetes(t *testing.T) {
	args := Attributes{
		Kubernetes: KubernetesDecorator{
			Enable:                   "true",
			KubeconfigPath:           "/etc/kube/config",
			ReconnectInitialInterval: 3 * time.Second,
			DropExternal:             true,
			ServiceNameTemplate:      "{{.Namespace}}/{{.Name}}",
			ResourceLabels:           map[string][]string{"service.name": {"app.kubernetes.io/name"}},
		},
	}
	cfg := args.Convert()
	require.Equal(t, "/etc/kube/config", cfg.Kubernetes.KubeconfigPath)
	require.Equal(t, 3*time.Second, cfg.Kubernetes.ReconnectInitialInterval)
	require.True(t, cfg.Kubernetes.DropExternal)
	require.Equal(t, "{{.Namespace}}/{{.Name}}", cfg.Kubernetes.ServiceNameTemplate)
	require.Equal(t, map[string][]string{"service.name": {"app.kubernetes.io/name"}}, map[string][]string(cfg.Kubernetes.ResourceLabels))
}

func TestConvert_Discovery_NewScalars(t *testing.T) {
	args := Discovery{
		PollInterval:        5 * time.Second,
		MinProcessAge:       2 * time.Second,
		DefaultOtlpGRPCPort: 4319,
		ExcludeOTelInstrumentedServicesSpanMetrics: true,
	}
	cfg, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, 5*time.Second, cfg.PollInterval)
	require.Equal(t, 2*time.Second, cfg.MinProcessAge)
	require.Equal(t, 4319, cfg.DefaultOtlpGRPCPort)
	require.True(t, cfg.ExcludeOTelInstrumentedServicesSpanMetrics)
}

func TestConvert_PayloadExtraction_Protocols(t *testing.T) {
	args := EBPF{
		PayloadExtraction: PayloadExtraction{
			HTTP: HTTPPayloadExtraction{
				GraphQL:       ProtocolToggle{Enabled: true},
				Elasticsearch: ProtocolToggle{Enabled: true},
				AWS:           ProtocolToggle{Enabled: true},
				JSONRPC:       ProtocolToggle{Enabled: true},
				SQLPP:         SQLPP{Enabled: true, EndpointPatterns: []string{"/query"}},
				GenAI: GenAI{
					OpenAI:    ProtocolToggle{Enabled: true},
					Anthropic: ProtocolToggle{Enabled: true},
					Gemini:    ProtocolToggle{Enabled: true},
					Qwen:      ProtocolToggle{Enabled: true},
					Bedrock:   ProtocolToggle{Enabled: true},
					MCP:       ProtocolToggle{Enabled: true},
					Embedding: ProtocolToggle{Enabled: true},
					Rerank:    ProtocolToggle{Enabled: true},
					Retrieval: ProtocolToggle{Enabled: true},
				},
			},
		},
	}
	cfg, err := args.Convert()
	require.NoError(t, err)
	h := cfg.PayloadExtraction.HTTP
	require.True(t, h.GraphQL.Enabled)
	require.True(t, h.Elasticsearch.Enabled)
	require.True(t, h.AWS.Enabled)
	require.True(t, h.JSONRPC.Enabled)
	require.True(t, h.SQLPP.Enabled)
	require.Equal(t, []string{"/query"}, h.SQLPP.EndpointPatterns)
	require.True(t, h.GenAI.OpenAI.Enabled)
	require.True(t, h.GenAI.Anthropic.Enabled)
	require.True(t, h.GenAI.Gemini.Enabled)
	require.True(t, h.GenAI.Qwen.Enabled)
	require.True(t, h.GenAI.Bedrock.Enabled)
	require.True(t, h.GenAI.MCP.Enabled)
	require.True(t, h.GenAI.Embedding.Enabled)
	require.True(t, h.GenAI.Rerank.Enabled)
	require.True(t, h.GenAI.Retrieval.Enabled)
}

func TestConvert_Enrichment(t *testing.T) {
	args := Enrichment{
		Enabled: true,
		Policy: EnrichmentPolicy{
			DefaultAction:     EnrichmentDefaultAction{Headers: "exclude", Body: "exclude"},
			ObfuscationString: "***",
		},
		Rules: []EnrichmentRule{
			{
				Action: "obfuscate", Type: "headers", Scope: "request",
				Match: EnrichmentMatch{
					Patterns:        []string{"authorization", "x-*"},
					CaseSensitive:   false,
					URLPathPatterns: []string{"/api/*"},
					Methods:         []string{"POST"},
				},
			},
			{
				Action: "obfuscate", Type: "body", Scope: "all",
				Match: EnrichmentMatch{ObfuscationJSONPaths: []string{"$.password"}},
			},
		},
	}
	got, err := args.Convert()
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, obiCfg.HTTPParsingActionExclude, got.Policy.DefaultAction.Headers)
	require.Equal(t, obiCfg.HTTPParsingActionExclude, got.Policy.DefaultAction.Body)
	require.Equal(t, "***", got.Policy.ObfuscationString)
	require.Len(t, got.Rules, 2)
	require.Equal(t, obiCfg.HTTPParsingActionObfuscate, got.Rules[0].Action)
	require.Equal(t, obiCfg.HTTPParsingRuleTypeHeaders, got.Rules[0].Type)
	require.Equal(t, obiCfg.HTTPParsingScopeRequest, got.Rules[0].Scope)
	require.Len(t, got.Rules[0].Match.Patterns, 2)
	require.True(t, got.Rules[0].Match.Patterns[0].IsSet())
	require.Len(t, got.Rules[0].Match.URLPathPatterns, 1)
	require.Equal(t, []obiCfg.HTTPMethod{obiCfg.HTTPMethodPOST}, got.Rules[0].Match.Methods)
	require.Equal(t, []string{"$.password"}, jsonPathsToStrings(got.Rules[1].Match.ObfuscationJSONPaths))
}

func jsonPathsToStrings(in []obiCfg.JSONPathExpr) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = in[i].String()
	}
	return out
}

func TestConvert_Enrichment_InvalidGlob(t *testing.T) {
	args := Enrichment{
		Enabled: true,
		Rules: []EnrichmentRule{
			{Action: "obfuscate", Type: "headers", Scope: "all",
				Match: EnrichmentMatch{Patterns: []string{"[invalid"}}},
		},
	}
	_, err := args.Convert()
	require.Error(t, err)
	require.Contains(t, err.Error(), "patterns")
}

func TestValidate_Enrichment_Enums(t *testing.T) {
	bad := Arguments{EBPF: EBPF{PayloadExtraction: PayloadExtraction{HTTP: HTTPPayloadExtraction{
		Enrichment: Enrichment{Rules: []EnrichmentRule{{Action: "bogus", Type: "headers", Scope: "all"}}},
	}}}}
	require.ErrorContains(t, bad.Validate(), "enrichment.rule[0].action")
}

func TestConvert_Metrics_Tuning(t *testing.T) {
	args := Metrics{
		ExemplarFilter:       "trace_based",
		TTL:                  5 * time.Minute,
		SpanServiceCacheSize: 1000,
		NativeHistogram: NativeHistogram{
			BucketFactor:     1.2,
			MaxBucketNumber:  120,
			MinResetDuration: 30 * time.Minute,
		},
		Buckets: Buckets{
			DurationHistogram:    []float64{0, 1, 2},
			RequestSizeHistogram: []float64{0, 100},
		},
	}
	cfg := args.Convert()
	require.Equal(t, "trace_based", cfg.ExemplarFilter)
	require.Equal(t, 5*time.Minute, cfg.TTL)
	require.Equal(t, 1000, cfg.SpanMetricsServiceCacheSize)
	require.Equal(t, 1.2, cfg.NativeHistogram.BucketFactor)
	require.Equal(t, uint32(120), cfg.NativeHistogram.MaxBucketNumber)
	require.Equal(t, 30*time.Minute, cfg.NativeHistogram.MinResetDuration)
	require.Equal(t, []float64{0, 1, 2}, cfg.Buckets.DurationHistogram)
	require.Equal(t, []float64{0, 100}, cfg.Buckets.RequestSizeHistogram)
}

func TestValidate_Metrics_ExemplarFilter(t *testing.T) {
	bad := Arguments{Metrics: Metrics{ExemplarFilter: "nope"}}
	require.ErrorContains(t, bad.Validate(), "metrics.exemplar_filter")
}

func TestConvert_GeoIP_ReverseDNS(t *testing.T) {
	geo := GeoIP{
		IPInfoPath:         "/data/ipinfo.mmdb",
		MaxMindCountryPath: "/data/country.mmdb",
		MaxMindASNPath:     "/data/asn.mmdb",
		CacheLen:           1000,
		CacheTTL:           time.Hour,
	}
	nc := beyla.DefaultConfig().NetworkFlows
	geo.applyToNetwork(&nc)
	require.Equal(t, "/data/ipinfo.mmdb", nc.GeoIP.IPInfo.Path)
	require.Equal(t, "/data/country.mmdb", nc.GeoIP.MaxMindInfo.CountryPath)
	require.Equal(t, "/data/asn.mmdb", nc.GeoIP.MaxMindInfo.ASNPath)
	require.Equal(t, 1000, nc.GeoIP.CacheLen)
	require.Equal(t, time.Hour, nc.GeoIP.CacheTTL)

	rdnsArgs := ReverseDNS{Type: "local", CacheLen: 256, CacheTTL: time.Minute}
	rdnsArgs.applyToNetwork(&nc)
	require.Equal(t, "local", nc.ReverseDNS.Type)
	require.Equal(t, 256, nc.ReverseDNS.CacheLen)
	require.Equal(t, time.Minute, nc.ReverseDNS.CacheTTL)
}

func TestConvert_Network_NewOptions(t *testing.T) {
	args := Network{
		Deduper:          "first_come",
		DeduperFCTTL:     30 * time.Second,
		GuessPorts:       "ordinal",
		ListenInterfaces: "poll",
		ListenPollPeriod: 10 * time.Second,
		PrintFlows:       true,
		GeoIP:            GeoIP{CacheLen: 10},
		ReverseDNS:       ReverseDNS{Type: "ebpf"},
	}
	cfg := args.Convert()
	require.Equal(t, "first_come", cfg.Deduper)
	require.Equal(t, 30*time.Second, cfg.DeduperFCTTL)
	require.Equal(t, "ordinal", string(cfg.GuessPorts))
	require.Equal(t, "poll", cfg.ListenInterfaces)
	require.Equal(t, 10*time.Second, cfg.ListenPollPeriod)
	require.True(t, cfg.Print)
	require.Equal(t, 10, cfg.GeoIP.CacheLen)
	require.Equal(t, "ebpf", cfg.ReverseDNS.Type)
}

func TestConvert_Stats_Enrichment(t *testing.T) {
	args := Stats{GeoIP: GeoIP{CacheLen: 5}, ReverseDNS: ReverseDNS{Type: "local"}}
	cfg := args.Convert()
	require.Equal(t, 5, cfg.GeoIP.CacheLen)
	require.Equal(t, "local", cfg.ReverseDNS.Type)
}

func TestValidate_Network_Enums(t *testing.T) {
	a1 := Arguments{Metrics: Metrics{Network: Network{Deduper: "bogus"}}}
	require.ErrorContains(t, a1.Validate(), "metrics.network.deduper")

	a2 := Arguments{Metrics: Metrics{Network: Network{GuessPorts: "bogus"}}}
	require.ErrorContains(t, a2.Validate(), "metrics.network.guess_ports")

	a3 := Arguments{Metrics: Metrics{Network: Network{ListenInterfaces: "bogus"}}}
	require.ErrorContains(t, a3.Validate(), "metrics.network.listen_interfaces")

	a4 := Arguments{Metrics: Metrics{Network: Network{ReverseDNS: ReverseDNS{Type: "bogus"}}}}
	require.ErrorContains(t, a4.Validate(), "metrics.network.reverse_dns.type")

	a5 := Arguments{Stats: Stats{ReverseDNS: ReverseDNS{Type: "bogus"}}}
	require.ErrorContains(t, a5.Validate(), "stats.reverse_dns.type")
}

func TestConvertGlob_LanguagesAndPIDs(t *testing.T) {
	svcs := Services{
		{Name: "app", Languages: "java", PIDs: []uint32{100, 200}},
	}
	crit, err := svcs.ConvertGlob()
	require.NoError(t, err)
	require.Len(t, crit, 1)
	require.True(t, crit[0].Languages.IsSet())
	require.Equal(t, []uint32{100, 200}, crit[0].PIDs)
}
