//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/beyla/v2/pkg/beyla"
	beylaSvc "github.com/grafana/beyla/v2/pkg/services"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/obi/pkg/appolly/services"
	"go.opentelemetry.io/obi/pkg/export/attributes"
	"go.opentelemetry.io/obi/pkg/export/debug"
	"go.opentelemetry.io/obi/pkg/export/instrumentations"
	"go.opentelemetry.io/obi/pkg/filter"
	"go.opentelemetry.io/obi/pkg/kube/kubeflags"
	"go.opentelemetry.io/obi/pkg/transform"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
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
	cfg, err := args.Convert()
	require.NoError(t, err)

	require.Equal(t, transform.UnmatchType("wildcard"), cfg.Routes.Unmatch)
	require.Equal(t, []string{"/api/v1/*"}, cfg.Routes.Patterns)
	require.Equal(t, []string{"/api/v1/health"}, cfg.Routes.IgnorePatterns)
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

	require.True(t, cfg.NetworkFlows.Enable)
	require.Equal(t, "0.0.0.0", cfg.NetworkFlows.AgentIP)
	require.Equal(t, []string{"eth0"}, cfg.NetworkFlows.Interfaces)
	require.Equal(t, []string{"TCP", "UDP"}, cfg.NetworkFlows.Protocols)
	require.Equal(t, []string{"ICMP"}, cfg.NetworkFlows.ExcludeProtocols)
	require.Equal(t, 1, cfg.NetworkFlows.Sampling)
	require.Equal(t, "10.0.0.0/8", cfg.NetworkFlows.CIDRs[0])
	require.Equal(t, 8000, cfg.NetworkFlows.CacheMaxFlows)
	require.Equal(t, 10*time.Second, cfg.NetworkFlows.CacheActiveTimeout)
	require.Equal(t, "ingress", cfg.NetworkFlows.Direction)
	require.Equal(t, "local", cfg.NetworkFlows.AgentIPIface)
	require.Equal(t, "ipv4", cfg.NetworkFlows.AgentIPType)
	require.Empty(t, cfg.NetworkFlows.ExcludeInterfaces)

	require.Len(t, cfg.Discovery.Instrument, 2)
	require.Equal(t, "test", cfg.Discovery.Instrument[0].Name)
	require.Equal(t, "default", cfg.Discovery.Instrument[0].Namespace)
	require.True(t, cfg.Discovery.Instrument[0].Path.IsSet())
	require.True(t, cfg.Discovery.Instrument[0].Metadata[services.AttrNamespace].IsSet())
	require.True(t, cfg.Discovery.Instrument[0].ExportModes.CanExportMetrics())
	require.True(t, cfg.Discovery.Instrument[0].ExportModes.CanExportTraces())
	require.Equal(t, &services.SamplerConfig{Name: "traceidratio", Arg: "0.5"}, cfg.Discovery.Instrument[0].SamplerConfig)
	require.True(t, cfg.Discovery.Instrument[1].PodLabels["test"].IsSet())
	require.True(t, cfg.Discovery.Instrument[1].ExportModes.CanExportMetrics())
	require.False(t, cfg.Discovery.Instrument[1].ExportModes.CanExportTraces())

	require.Len(t, cfg.Discovery.ExcludeInstrument, 1)
	require.True(t, cfg.Discovery.ExcludeInstrument[0].Path.IsSet())
	require.Equal(t, "default", cfg.Discovery.ExcludeInstrument[0].Namespace)

	require.Len(t, cfg.Discovery.Survey, 1)
	require.True(t, cfg.Discovery.Survey[0].Path.IsSet())
	require.Equal(t, "microservice", cfg.Discovery.Survey[0].Name)
	require.True(t, cfg.Discovery.Survey[0].ExportModes.CanExportMetrics())
	require.True(t, cfg.Discovery.Survey[0].ExportModes.CanExportTraces())

	require.Equal(t, []string{"application", "network"}, cfg.Prometheus.Features)
	require.Equal(t, []string{"redis", "sql", "gpu", "mongo"}, cfg.Prometheus.Instrumentations)

	require.True(t, cfg.EnforceSysCaps)
	require.Equal(t, 10, cfg.EBPF.WakeupLen)
	require.True(t, cfg.EBPF.TrackRequestHeaders)
	require.Equal(t, cfg.EBPF.ContextPropagation, obiCfg.ContextPropagationIPOptionsOnly)
	require.Equal(t, 10*time.Second, cfg.EBPF.HTTPRequestTimeout)
	require.True(t, cfg.EBPF.HighRequestVolume)
	require.True(t, cfg.EBPF.HeuristicSQLDetect)
	require.False(t, cfg.EBPF.BpfDebug)
	require.False(t, cfg.EBPF.ProtocolDebug)
	require.Len(t, cfg.Filters.Application, 1)
	require.Len(t, cfg.Filters.Network, 1)
	require.Equal(t, filter.MatchDefinition{NotMatch: "UDP"}, cfg.Filters.Application["transport"])
	require.Equal(t, filter.MatchDefinition{Match: "53"}, cfg.Filters.Network["dst_port"])
	require.Equal(t, debug.TracePrinter("json"), cfg.TracePrinter)
	require.Equal(t, []string{"http", "grpc", "kafka"}, cfg.TracesReceiver.Instrumentations)
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
			wantErr: `invalid port range "-8000". Must be a comma-separated list of numeric ports or port ranges (e.g. 8000-8999)`,
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
			Enable:                kubeflags.EnableFlag(args.Kubernetes.Enable),
			InformersSyncTimeout:  15 * time.Second,
			InformersResyncPeriod: 30 * time.Minute,
			ResourceLabels:        beyla.DefaultConfig().Attributes.Kubernetes.ResourceLabels,
			MetaCacheAddress:      "localhost:9090",
		},
		HostID: beyla.HostIDConfig{
			FetchTimeout: 500 * time.Millisecond,
		},
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
	require.Equal(t, "test", config.Instrument[0].Name)
	require.Equal(t, "default", config.Instrument[0].Namespace)
	require.Equal(t, services.PortEnum{Ranges: []services.PortRange{{Start: 80, End: 0}}}, config.Instrument[0].OpenPorts)
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
	require.Equal(t, "test", config.ExcludeInstrument[0].Name)
	require.Equal(t, "default", config.ExcludeInstrument[0].Namespace)
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
	expectedConfig.Features = args.Features
	expectedConfig.Instrumentations = args.Instrumentations
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
	expectedConfig.Features = args.Features
	expectedConfig.Instrumentations = args.Instrumentations
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
	expectedConfig.Enable = true
	expectedConfig.AgentIP = "0.0.0.0"
	expectedConfig.Interfaces = args.Interfaces
	expectedConfig.Protocols = args.Protocols
	expectedConfig.ExcludeProtocols = args.ExcludeProtocols
	expectedConfig.Sampling = 1
	expectedConfig.Print = false
	expectedConfig.CIDRs = args.CIDRs

	config := args.Convert(true)

	require.Equal(t, expectedConfig, config)
}

func TestConvert_EBPF(t *testing.T) {
	args := EBPF{
		WakeupLen:           10,
		TrackRequestHeaders: true,
		HighRequestVolume:   true,
		HeuristicSQLDetect:  true,
		ContextPropagation:  "headers",
		BpfDebug:            true,
		ProtocolDebug:       true,
	}

	expectedConfig := beyla.DefaultConfig().EBPF
	expectedConfig.WakeupLen = 10
	expectedConfig.TrackRequestHeaders = true
	expectedConfig.HighRequestVolume = true
	expectedConfig.HeuristicSQLDetect = true
	expectedConfig.ContextPropagation = obiCfg.ContextPropagationHeadersOnly
	expectedConfig.BpfDebug = true
	expectedConfig.ProtocolDebug = true

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
	var buf bytes.Buffer
	var mu sync.Mutex

	// Create a synchronized logger that protects both writing and reading
	syncLogger := log.LoggerFunc(func(keyvals ...any) error {
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

	ctx, cancel := context.WithCancel(t.Context())
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
				Instrumentations: []string{
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
				Instrumentations: []string{"http", "grpc"},
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
				Instrumentations: []string{"kafka"},
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
				require.Equal(t, tt.expectedSamplerName, result[0].SamplerConfig.Name)
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
