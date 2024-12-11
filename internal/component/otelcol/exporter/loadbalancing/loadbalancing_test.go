package loadbalancing_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/loadbalancing"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
)

func getPtrToUint(v uint16) *uint16 {
	res := &v
	return res
}

// Test performs a basic integration test which runs the otelcol.exporter.loadbalancing
// component and ensures that it can pass data to an OTLP gRPC server.
func Test(t *testing.T) {
	traceCh := make(chan ptrace.Traces)
	tracesServer := makeTracesServer(t, traceCh)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.loadbalancing")
	require.NoError(t, err)

	cfgTemplate := `
			routing_key = "%s"
			resolver {
				static {
					hostnames = ["%s"]
				}
			}
			protocol {
				otlp {
					client {
						compression = "none"

						tls {
							insecure             = true
							insecure_skip_verify = true
						}
					}
				}
			}

			debug_metrics {
				disable_high_cardinality_metrics = true
			}
		`

	cfg := fmt.Sprintf(cfgTemplate, "traceID", tracesServer)
	var args loadbalancing.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	require.Equal(t, args.DebugMetricsConfig().DisableHighCardinalityMetrics, true)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	// Send traces in the background to our exporter.
	go func() {
		exports := ctrl.Exports().(otelcol.ConsumerExports)

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			err := exports.Input.ConsumeTraces(ctx, createTestTraces())
			if err != nil {
				level.Error(l).Log("msg", "failed to send traces", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our exporter to finish and pass data to our rpc server.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for traces")
	case tr := <-traceCh:
		require.Equal(t, 1, tr.SpanCount())
	}

	// Update the config to disable traces export
	cfg = fmt.Sprintf(cfgTemplate, "metric", tracesServer)
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	ctrl.Update(args)

	// Send traces in the background to our exporter.
	go func() {
		exports := ctrl.Exports().(otelcol.ConsumerExports)

		bo := backoff.New(ctx, backoff.Config{
			MaxRetries: 3,
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			err := exports.Input.ConsumeTraces(ctx, createTestTraces())
			require.ErrorContains(t, err, "telemetry type is not supported")
			if err != nil {
				level.Error(l).Log("msg", "failed to send traces", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our exporter to finish and pass data to our rpc server.
	// no error here, as we we expect to fail sending in the first place
	select {
	case <-traceCh:
		require.FailNow(t, "no traces expected here")
	case <-time.After(time.Second):
	}

	// Re-run the test with reenabled traces export
	cfg = fmt.Sprintf(cfgTemplate, "traceID", tracesServer)
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	ctrl.Update(args)

	// Send traces in the background to our exporter.
	go func() {
		exports := ctrl.Exports().(otelcol.ConsumerExports)

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			err := exports.Input.ConsumeTraces(ctx, createTestTraces())
			if err != nil {
				level.Error(l).Log("msg", "failed to send traces", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our exporter to finish and pass data to our rpc server.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for traces")
	case tr := <-traceCh:
		require.Equal(t, 1, tr.SpanCount())
	}
}

// makeTracesServer returns a host:port which will accept traces over insecure
// gRPC.
func makeTracesServer(t *testing.T, ch chan ptrace.Traces) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	ptraceotlp.RegisterGRPCServer(srv, &mockTracesReceiver{ch: ch})

	go func() {
		err := srv.Serve(lis)
		require.NoError(t, err)
	}()
	t.Cleanup(srv.Stop)

	return lis.Addr().String()
}

type mockTracesReceiver struct {
	ptraceotlp.UnimplementedGRPCServer
	ch chan ptrace.Traces
}

var _ ptraceotlp.GRPCServer = (*mockTracesReceiver)(nil)

func (ms *mockTracesReceiver) Export(_ context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	ms.ch <- req.Traces()
	return ptraceotlp.NewExportResponse(), nil
}

func createTestTraces() ptrace.Traces {
	// Matches format from the protobuf definition:
	// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto
	bb := `{
			"resource_spans": [{
				"scope_spans": [{
					"spans": [{
						"name": "TestSpan"
					}]
				}]
			}]
		}`

	decoder := &ptrace.JSONUnmarshaler{}
	data, err := decoder.UnmarshalTraces([]byte(bb))
	if err != nil {
		panic(err)
	}
	return data
}

func TestConfigConversion(t *testing.T) {
	var (
		defaultRetrySettings = configretry.NewDefaultBackOffConfig()
		defaultTimeoutConfig = exporterhelper.NewDefaultTimeoutConfig()

		defaultQueueSettings = exporterhelper.QueueConfig{
			Enabled:      true,
			NumConsumers: 10,
			QueueSize:    1000,
		}

		defaultProtocol = loadbalancingexporter.Protocol{
			OTLP: otlpexporter.Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint:        "",
					Compression:     "gzip",
					WriteBufferSize: 512 * 1024,
					Headers:         map[string]configopaque.String{},
					BalancerName:    otelcol.DefaultBalancerName,
				},
				RetryConfig:   defaultRetrySettings,
				TimeoutConfig: defaultTimeoutConfig,
				QueueConfig:   defaultQueueSettings,
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
						TimeoutConfig: exporterhelper.TimeoutConfig{
							Timeout: 1 * time.Second,
						},
						RetryConfig: defaultRetrySettings,
						QueueConfig: defaultQueueSettings,
						ClientConfig: configgrpc.ClientConfig{
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
						Timeout: 1 * time.Second,
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
					timeout = "13s"
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
						Timeout: 13 * time.Second,
					},
				},
				RoutingKey: "traceID",
				Protocol:   defaultProtocol,
			},
		},
		{
			testName: "aws with defaults",
			alloyCfg: `
			resolver {
				aws_cloud_map {
					namespace = "cloudmap"
					service_name = "otelcollectors"
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
					K8sSvc: nil,
					AWSCloudMap: &loadbalancingexporter.AWSCloudMapResolver{
						NamespaceName: "cloudmap",
						ServiceName:   "otelcollectors",
						HealthStatus:  "HEALTHY",
						Interval:      30 * time.Second,
						Timeout:       5 * time.Second,
						Port:          nil,
					},
				},
				RoutingKey: "traceID",
				Protocol:   defaultProtocol,
			},
		},
		{
			testName: "aws with non-defaults",
			alloyCfg: `
				resolver {
					aws_cloud_map {
						namespace = "cloudmap3"
						service_name = "otelcollectors3"
						health_status = "UNHEALTHY"
						interval = "123s"
						timeout = "113s"
						port = 4321
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
					K8sSvc: nil,
					AWSCloudMap: &loadbalancingexporter.AWSCloudMapResolver{
						NamespaceName: "cloudmap3",
						ServiceName:   "otelcollectors3",
						HealthStatus:  "UNHEALTHY",
						Interval:      123 * time.Second,
						Timeout:       113 * time.Second,
						Port:          getPtrToUint(4321),
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
		expected otelcolCfg.DebugMetricsArguments
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
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
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
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
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
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
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
