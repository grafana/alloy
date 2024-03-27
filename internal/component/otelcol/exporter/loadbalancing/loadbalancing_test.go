package loadbalancing_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/loadbalancing"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

func TestConfigConversion(t *testing.T) {
	var (
		defaultRetrySettings   = exporterhelper.NewDefaultRetrySettings()
		defaultTimeoutSettings = exporterhelper.NewDefaultTimeoutSettings()

		defaultQueueSettings = exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 10,
			QueueSize:    1000,
		}

		defaultProtocol = loadbalancingexporter.Protocol{
			OTLP: otlpexporter.Config{
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Endpoint:        "",
					Compression:     "gzip",
					WriteBufferSize: 512 * 1024,
					Headers:         map[string]configopaque.String{},
					BalancerName:    otelcol.DefaultBalancerName,
				},
				RetrySettings:   defaultRetrySettings,
				TimeoutSettings: defaultTimeoutSettings,
				QueueSettings:   defaultQueueSettings,
			},
		}
	)

	tests := []struct {
		testName string
		alloyCfg string
		expected loadbalancingexporter.Config
	}{
		{
			testName: "static",
			alloyCfg: `
			resolver {
				static {
					hostnames = ["endpoint-1"]
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			`,
			expected: loadbalancingexporter.Config{
				Resolver: loadbalancingexporter.ResolverSettings{
					Static: &loadbalancingexporter.StaticResolver{
						Hostnames: []string{"endpoint-1"},
					},
					DNS: nil,
				},
				RoutingKey: "traceID",
				Protocol:   defaultProtocol,
			},
		},
		{
			testName: "static with service routing",
			alloyCfg: `
			routing_key = "service"
			resolver {
				static {
					hostnames = ["endpoint-1"]
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			`,
			expected: loadbalancingexporter.Config{
				Resolver: loadbalancingexporter.ResolverSettings{
					Static: &loadbalancingexporter.StaticResolver{
						Hostnames: []string{"endpoint-1"},
					},
					DNS: nil,
				},
				RoutingKey: "service",
				Protocol:   defaultProtocol,
			},
		},
		{
			testName: "static with timeout",
			alloyCfg: `
			protocol {
				otlp {
					timeout = "1s"
					client {
						authority = "authority"
					}
				}
			}
			resolver {
				static {
					hostnames = ["endpoint-1", "endpoint-2:55678"]
				}
			}`,
			expected: loadbalancingexporter.Config{
				Protocol: loadbalancingexporter.Protocol{
					OTLP: otlpexporter.Config{
						TimeoutSettings: exporterhelper.TimeoutSettings{
							Timeout: 1 * time.Second,
						},
						RetrySettings: defaultRetrySettings,
						QueueSettings: defaultQueueSettings,
						GRPCClientSettings: configgrpc.GRPCClientSettings{
							Endpoint:        "",
							Compression:     "gzip",
							WriteBufferSize: 512 * 1024,
							Headers:         map[string]configopaque.String{},
							BalancerName:    otelcol.DefaultBalancerName,
							Authority:       "authority",
						},
					},
				},
				Resolver: loadbalancingexporter.ResolverSettings{
					Static: &loadbalancingexporter.StaticResolver{
						Hostnames: []string{"endpoint-1", "endpoint-2:55678"},
					},
					DNS: nil,
				},
				RoutingKey: "traceID",
			},
		},
		{
			testName: "dns with defaults",
			alloyCfg: `
			resolver {
				dns {
					hostname = "service-1"
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			`,
			expected: loadbalancingexporter.Config{
				Resolver: loadbalancingexporter.ResolverSettings{
					Static: nil,
					DNS: &loadbalancingexporter.DNSResolver{
						Hostname: "service-1",
						Port:     "4317",
						Interval: 5 * time.Second,
						Timeout:  1 * time.Second,
					},
				},
				RoutingKey: "traceID",
				Protocol:   defaultProtocol,
			},
		},
		{
			testName: "dns with non-defaults",
			alloyCfg: `
			resolver {
				dns {
					hostname = "service-1"
					port = "55690"
					interval = "123s"
					timeout = "321s"
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			`,
			expected: loadbalancingexporter.Config{
				Resolver: loadbalancingexporter.ResolverSettings{
					Static: nil,
					DNS: &loadbalancingexporter.DNSResolver{
						Hostname: "service-1",
						Port:     "55690",
						Interval: 123 * time.Second,
						Timeout:  321 * time.Second,
					},
				},
				RoutingKey: "traceID",
				Protocol:   defaultProtocol,
			},
		},
		{
			testName: "k8s with defaults",
			alloyCfg: `
			resolver {
				kubernetes {
					service = "lb-svc.lb-ns"
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			`,
			expected: loadbalancingexporter.Config{
				Resolver: loadbalancingexporter.ResolverSettings{
					Static: nil,
					K8sSvc: &loadbalancingexporter.K8sSvcResolver{
						Service: "lb-svc.lb-ns",
						Ports:   []int32{4317},
					},
				},
				RoutingKey: "traceID",
				Protocol:   defaultProtocol,
			},
		},
		{
			testName: "k8s with non-defaults",
			alloyCfg: `
			resolver {
				kubernetes {
					service = "lb-svc.lb-ns"
					ports = [55690, 55691]
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			`,
			expected: loadbalancingexporter.Config{
				Resolver: loadbalancingexporter.ResolverSettings{
					Static: nil,
					K8sSvc: &loadbalancingexporter.K8sSvcResolver{
						Service: "lb-svc.lb-ns",
						Ports:   []int32{55690, 55691},
					},
				},
				RoutingKey: "traceID",
				Protocol:   defaultProtocol,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args loadbalancing.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			actual, err := args.Convert()
			require.NoError(t, err)
			require.Equal(t, &tc.expected, actual.(*loadbalancingexporter.Config))
		})
	}
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected otelcol.DebugMetricsArguments
	}{
		{
			testName: "default",
			alloyCfg: `
			resolver {
				static {
					hostnames = ["endpoint-1"]
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
		{
			testName: "explicit_false",
			alloyCfg: `
			resolver {
				static {
					hostnames = ["endpoint-1"]
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
			},
		},
		{
			testName: "explicit_true",
			alloyCfg: `
			resolver {
				static {
					hostnames = ["endpoint-1"]
				}
			}
			protocol {
				otlp {
					client {}
				}
			}
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args loadbalancing.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
