//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/beyla/v2/pkg/beyla"
	"github.com/grafana/beyla/v2/pkg/export/attributes"
	"github.com/grafana/beyla/v2/pkg/filter"
	"github.com/grafana/beyla/v2/pkg/kubeflags"
	"github.com/grafana/beyla/v2/pkg/services"
	"github.com/grafana/beyla/v2/pkg/transform"
	"github.com/stretchr/testify/require"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component"
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
		debug = true
		attributes {
			kubernetes {
				enable = "true"
				informers_sync_timeout = "15s"
				informers_resync_period = "30m"
				cluster_name = "test"
				disable_informers = ["node"]
				meta_restrict_local_node = true
			}
			select {
				attr = "sql_client_duration"
				include = ["*"]
				exclude = ["db_statement"]
			}
		}
		discovery {
			services {
				name = "test"
				namespace = "default"
				open_ports = "80,443"
				kubernetes {
					namespace = "default"
				}
			}
			services {
				name = "test2"
				namespace = "default"
				open_ports = "80,443"
				kubernetes {
					pod_labels = {
						test = "test",
					}
				}
			}
			exclude_services {
				exe_path = "test3"
				namespace = "default"
			}
		}
		metrics {
			features = ["application", "network"]
			instrumentations = ["redis", "sql"]
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
		ebpf {
			wakeup_len = 10
			track_request_headers = true
			enable_context_propagation = true
			http_request_timeout = "10s"
			high_request_volume = true
			heuristic_sql_detect = true
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
	require.Len(t, cfg.Discovery.Services, 2)
	require.Equal(t, "test", cfg.Discovery.Services[0].Name)
	require.Equal(t, "default", cfg.Discovery.Services[0].Namespace)
	require.True(t, cfg.Discovery.Services[0].Metadata[services.AttrNamespace].IsSet())
	require.True(t, cfg.Discovery.Services[1].PodLabels["test"].IsSet())
	require.Len(t, cfg.Discovery.ExcludeServices, 1)
	require.True(t, cfg.Discovery.ExcludeServices[0].Path.IsSet())
	require.Equal(t, "default", cfg.Discovery.ExcludeServices[0].Namespace)
	require.Equal(t, []string{"application", "network"}, cfg.Prometheus.Features)
	require.Equal(t, []string{"redis", "sql"}, cfg.Prometheus.Instrumentations)
	require.True(t, cfg.EnforceSysCaps)
	require.Equal(t, 10, cfg.EBPF.WakeupLen)
	require.True(t, cfg.EBPF.TrackRequestHeaders)
	require.True(t, cfg.EBPF.ContextPropagationEnabled)
	require.Equal(t, 10*time.Second, cfg.EBPF.HTTPRequestTimeout)
	require.True(t, cfg.EBPF.HighRequestVolume)
	require.True(t, cfg.EBPF.HeuristicSQLDetect)
	require.Len(t, cfg.Filters.Application, 1)
	require.Len(t, cfg.Filters.Network, 1)
	require.Equal(t, filter.MatchDefinition{NotMatch: "UDP"}, cfg.Filters.Application["transport"])
	require.Equal(t, filter.MatchDefinition{Match: "53"}, cfg.Filters.Network["dst_port"])
}

func TestArguments_ConvertDefaultConfig(t *testing.T) {
	args := Arguments{}
	cfg, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, cfg.ChannelBufferLen, beyla.DefaultConfig.ChannelBufferLen)
	require.Equal(t, cfg.LogLevel, beyla.DefaultConfig.LogLevel)
	require.Equal(t, cfg.EBPF, beyla.DefaultConfig.EBPF)
	require.Equal(t, cfg.NetworkFlows, beyla.DefaultConfig.NetworkFlows)
	require.Equal(t, cfg.Grafana, beyla.DefaultConfig.Grafana)
	require.Equal(t, cfg.Attributes, beyla.DefaultConfig.Attributes)
	require.Equal(t, cfg.Routes, beyla.DefaultConfig.Routes)
	require.Equal(t, cfg.Metrics, beyla.DefaultConfig.Metrics)
	require.Equal(t, cfg.Traces, beyla.DefaultConfig.Traces)
	require.Equal(t, cfg.Prometheus, beyla.DefaultConfig.Prometheus)
	require.Equal(t, cfg.InternalMetrics, beyla.DefaultConfig.InternalMetrics)
	require.Equal(t, cfg.NetworkFlows, beyla.DefaultConfig.NetworkFlows)
	require.Equal(t, cfg.Discovery, beyla.DefaultConfig.Discovery)
	require.Equal(t, cfg.EnforceSysCaps, beyla.DefaultConfig.EnforceSysCaps)
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
			wantErr: "error parsing regexp: missing closing ]: `[`",
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

func TestConvert_Routes(t *testing.T) {
	args := Routes{
		Unmatch:        "wildcard",
		Patterns:       []string{"/api/v1/*"},
		IgnorePatterns: []string{"/api/v1/health"},
		IgnoredEvents:  "all",
	}

	expectedConfig := &transform.RoutesConfig{
		Unmatch:        transform.UnmatchType(args.Unmatch),
		Patterns:       args.Patterns,
		IgnorePatterns: args.IgnorePatterns,
		IgnoredEvents:  transform.IgnoreMode(args.IgnoredEvents),
		WildcardChar:   "*",
	}

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_Attributes(t *testing.T) {
	args := Attributes{
		Kubernetes: KubernetesDecorator{
			Enable:               "true",
			InformersSyncTimeout: 15 * time.Second,
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
			ResourceLabels:        beyla.DefaultConfig.Attributes.Kubernetes.ResourceLabels,
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
	}
	expectedConfig.InstanceID.OverrideHostname = "test"
	expectedConfig.InstanceID.HostnameDNSResolution = true

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_Discovery(t *testing.T) {
	args := Discovery{
		Services: []Service{
			{
				Name:      "test",
				Namespace: "default",
				OpenPorts: "80",
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
				},
			},
		},
		ExcludeServices: []Service{
			{
				Name:      "test",
				Namespace: "default",
			},
		},
		DefaultExcludeServices: []Service{},
	}
	config, err := args.Convert()

	require.NoError(t, err)
	require.Len(t, config.Services, 3)
	require.Equal(t, "test", config.Services[0].Name)
	require.Equal(t, "default", config.Services[0].Namespace)
	require.Equal(t, services.PortEnum{Ranges: []services.PortRange{{Start: 80, End: 0}}}, config.Services[0].OpenPorts)
	require.True(t, config.Services[1].Metadata[services.AttrNamespace].IsSet())
	require.True(t, config.Services[1].Metadata[services.AttrDeploymentName].IsSet())
	_, exists := config.Services[1].Metadata[services.AttrDaemonSetName]
	require.False(t, exists)
	require.True(t, config.Services[2].Metadata[services.AttrNamespace].IsSet())
	require.True(t, config.Services[2].Metadata[services.AttrPodName].IsSet())
	require.True(t, config.Services[2].Metadata[services.AttrDeploymentName].IsSet())
	require.True(t, config.Services[2].Metadata[services.AttrReplicaSetName].IsSet())
	require.True(t, config.Services[2].Metadata[services.AttrStatefulSetName].IsSet())
	require.True(t, config.Services[2].Metadata[services.AttrDaemonSetName].IsSet())
	require.True(t, config.Services[2].Metadata[services.AttrOwnerName].IsSet())
	require.True(t, config.Services[2].PodLabels["test"].IsSet())
	require.NoError(t, config.Services.Validate())
	require.Len(t, config.ExcludeServices, 1)
	require.Equal(t, "test", config.ExcludeServices[0].Name)
	require.Equal(t, "default", config.ExcludeServices[0].Namespace)
	require.Equal(t, true, config.ExcludeOTelInstrumentedServices)
	require.Empty(t, config.DefaultExcludeServices)
}

func TestConvert_Prometheus(t *testing.T) {
	args := Metrics{
		Features:                        []string{"application", "network"},
		Instrumentations:                []string{"redis", "sql"},
		AllowServiceGraphSelfReferences: true,
	}

	expectedConfig := beyla.DefaultConfig.Prometheus
	expectedConfig.Features = args.Features
	expectedConfig.Instrumentations = args.Instrumentations
	expectedConfig.AllowServiceGraphSelfReferences = true

	config := args.Convert()

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

	expectedConfig := beyla.DefaultConfig.NetworkFlows
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
	}

	expectedConfig := beyla.DefaultConfig.EBPF
	expectedConfig.WakeupLen = 10
	expectedConfig.TrackRequestHeaders = true
	expectedConfig.HighRequestVolume = true
	expectedConfig.HeuristicSQLDetect = true
	expectedConfig.ContextPropagationEnabled = false

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
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
				Instrumentations: []string{"http", "grpc", "*"},
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
			name: "valid features",
			args: Metrics{
				Features: []string{"application", "network"},
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
			name:    "empty arguments",
			args:    Arguments{},
			wantErr: "metrics.features must include at least one of: network, application",
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
					Services: Services{}, // Empty services
				},
			},
			wantErr: "discovery.services is required when application features are enabled",
		},
		{
			name: "valid application configuration",
			args: Arguments{
				Discovery: Discovery{
					Services: Services{
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
						{}, // Empty service
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
			wantErr: "metrics.features must include at least one of: network, application, application_span, application_service_graph, or application_process",
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
	var (
		buf bytes.Buffer
		mu  sync.Mutex // Add mutex to protect buffer access
	)

	logger := level.NewFilter(log.NewLogfmtLogger(&buf), level.AllowAll())

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
		output := buf.String()
		mu.Unlock()
		return strings.Contains(output, "level=warn") &&
			strings.Contains(output, "open_port' field is deprecated") &&
			strings.Contains(output, "executable_name' field is deprecated")
	}, time.Second, time.Millisecond*10)
}
