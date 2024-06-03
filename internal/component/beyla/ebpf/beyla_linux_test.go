//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"errors"
	"testing"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/beyla/pkg/beyla"
	"github.com/grafana/beyla/pkg/services"
	"github.com/grafana/beyla/pkg/transform"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalSyntax(t *testing.T) {
	in := `
		open_port = "80,443,8000-8999"
		executable_name = "test"
		routes {
			unmatched = "wildcard"
			patterns = ["/api/v1/*"]
			ignored_patterns = ["/api/v1/health"]
			ignore_mode = "all"
		}
		attributes {
			kubernetes {
				enable = "true"
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
		}
		output { /* no-op */ }
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(in), &args))
	cfg, err := args.Convert()
	require.NoError(t, err)
	require.Equal(t, services.PortEnum{Ranges: []services.PortRange{{Start: 80, End: 0}, {Start: 443, End: 0}, {Start: 8000, End: 8999}}}, cfg.Port)
	require.True(t, cfg.Exec.IsSet())
	require.Equal(t, transform.UnmatchType("wildcard"), cfg.Routes.Unmatch)
	require.Equal(t, []string{"/api/v1/*"}, cfg.Routes.Patterns)
	require.Equal(t, []string{"/api/v1/health"}, cfg.Routes.IgnorePatterns)
	require.Equal(t, transform.IgnoreMode("all"), cfg.Routes.IgnoredEvents)
	require.Equal(t, transform.KubeEnableFlag("true"), cfg.Attributes.Kubernetes.Enable)
	require.Len(t, cfg.Discovery.Services, 2)
	require.Equal(t, "test", cfg.Discovery.Services[0].Name)
	require.Equal(t, "default", cfg.Discovery.Services[0].Namespace)
	require.True(t, cfg.Discovery.Services[0].Metadata[services.AttrNamespace].IsSet())
	require.True(t, cfg.Discovery.Services[1].PodLabels["test"].IsSet())
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
}

func TestArguments_UnmarshalInvalidSyntax(t *testing.T) {
	var tests = []struct {
		testname      string
		cfg           string
		expectedError string
	}{
		{
			"invalid regex",
			`
		executable_name = "["
		`,
			"error parsing regexp: missing closing ]: `[`",
		},
		{
			"invalid port range",
			`
		open_port = "-8000"
		`,
			"invalid port range \"-8000\". Must be a comma-separated list of numeric ports or port ranges (e.g. 8000-8999)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			var args Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tt.cfg), &args))
			_, err := args.Convert()
			require.EqualError(t, err, tt.expectedError)
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
	}

	config := args.Convert()

	require.Equal(t, expectedConfig, config)
}

func TestConvert_Attribute(t *testing.T) {
	args := Attributes{
		Kubernetes: KubernetesDecorator{
			Enable: "true",
		},
	}

	expectedConfig := beyla.Attributes{
		InstanceID: beyla.DefaultConfig.Attributes.InstanceID,
		Kubernetes: transform.KubernetesDecorator{
			Enable:               transform.KubeEnableFlag(args.Kubernetes.Enable),
			InformersSyncTimeout: 30 * time.Second,
		},
	}

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
				Path:      "/api/v1/*",
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
	}
	config, err := args.Convert()

	require.NoError(t, err)
	require.Len(t, config.Services, 1)
	require.Equal(t, "test", config.Services[0].Name)
	require.Equal(t, "default", config.Services[0].Namespace)
	require.Equal(t, services.PortEnum{Ranges: []services.PortRange{{Start: 80, End: 0}}}, config.Services[0].OpenPorts)
	require.True(t, config.Services[0].Path.IsSet())
	require.True(t, config.Services[0].Metadata[services.AttrNamespace].IsSet())
	require.True(t, config.Services[0].Metadata[services.AttrPodName].IsSet())
	require.True(t, config.Services[0].Metadata[services.AttrDeploymentName].IsSet())
	require.True(t, config.Services[0].Metadata[services.AttrReplicaSetName].IsSet())
	require.True(t, config.Services[0].Metadata[services.AttrStatefulSetName].IsSet())
	require.True(t, config.Services[0].Metadata[services.AttrDaemonSetName].IsSet())
	require.True(t, config.Services[0].Metadata[services.AttrOwnerName].IsSet())
	require.True(t, config.Services[0].PodLabels["test"].IsSet())
	require.NoError(t, config.Services.Validate())
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name     string
		args     Arguments
		expected error
	}{
		{
			name:     "empty arguments",
			args:     Arguments{},
			expected: errors.New("you need to define at least open_port, executable_name, or services in the discovery section"),
		},
		{
			name: "with service discovery",
			args: Arguments{
				Discovery: Discovery{
					Services: []Service{
						{
							Name:      "test",
							Namespace: "default",
							OpenPorts: "80",
							Path:      "/api/v1/*",
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "with port",
			args: Arguments{
				Port: "80",
			},
			expected: nil,
		},
		{
			name: "with executable name",
			args: Arguments{
				ExecutableName: "test",
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			require.Equal(t, tt.expected, err)
		})
	}
}
