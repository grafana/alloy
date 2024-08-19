package jaeger_remote_sampling_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension/jaeger_remote_sampling"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

// Test performs a basic integration test which runs the otelcol.extension.jaeger_remote_sampling
// component and ensures that it can be used for authentication.
func TestFileSource(t *testing.T) {
	// write remote sampling config to a temp file
	remoteSamplingConfig := `
	{
		"default_strategy": {
		  "type": "probabilistic",
		  "param": 0.5
		}
	}
	`
	// The strategy type is not returned by proto-gen/api_v2 if set to probabilistic.
	expectedRemoteSamplingConfig := `
	{
		"probabilisticSampling": {
			"samplingRate": 0.5
		}
	}
	`

	remoteSamplingConfigFile := filepath.ToSlash(filepath.Join(t.TempDir(), "remote.json"))
	err := os.WriteFile(remoteSamplingConfigFile, []byte(remoteSamplingConfig), 0644)
	require.NoError(t, err)

	listenAddr := getFreeAddr(t)
	cfg := fmt.Sprintf(`
	    http {
			endpoint = "%s"
	    }
		source {
			file = "%s"
		}
	`, listenAddr, remoteSamplingConfigFile)

	get, cancel := startJaegerRemoteSamplingServer(t, cfg, listenAddr)
	defer cancel()

	actual := get("foo")
	require.JSONEq(t, actual, expectedRemoteSamplingConfig)
}

func TestContentSource(t *testing.T) {
	// write remote sampling config to a temp file
	remoteSamplingConfig := `{ \"default_strategy\": {\"type\": \"probabilistic\", \"param\": 0.5 } }`
	// The strategy type is not returned by proto-gen/api_v2 if set to probabilistic.
	expectedRemoteSamplingConfig := `
	{
		"probabilisticSampling": {
			"samplingRate": 0.5
		}
	}
	`
	listenAddr := getFreeAddr(t)
	cfg := fmt.Sprintf(`
	    http {
			endpoint = "%s"
	    }
		source {
			content = "%s"
		}
	`, listenAddr, remoteSamplingConfig)

	get, cancel := startJaegerRemoteSamplingServer(t, cfg, listenAddr)
	defer cancel()

	actual := get("foo")
	require.JSONEq(t, actual, expectedRemoteSamplingConfig)
}

func startJaegerRemoteSamplingServer(t *testing.T, cfg string, listenAddr string) (func(svc string) string, context.CancelFunc) {
	ctx := componenttest.TestContext(t)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)

	l := util.TestLogger(t)

	// Create and run our component
	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.extension.jaeger_remote_sampling")
	require.NoError(t, err)

	var args jaeger_remote_sampling.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	// the wrapped jaeger remote sampler starts its http server async: ./internal/jaegerremotesampling/internal/http.go
	// and reports errors back through ReportFatalError. Since we can't wait on this server directly just pause for a bit here while it starts up
	util.Eventually(t, func(t require.TestingT) {
		_, err := http.Get("http://" + listenAddr + "/sampling?service=foo")
		require.NoError(t, err)
	})

	return func(svc string) string {
		resp, err := http.Get("http://" + listenAddr + "/sampling?service=" + svc)
		require.NoError(t, err, "HTTP request failed")
		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(b)
	}, cancel
}

func TestUnmarshalFailsWithNoServerConfig(t *testing.T) {
	cfg := `
		source {
			file = "remote.json"
		}
	`

	var args jaeger_remote_sampling.Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.ErrorContains(t, err, "http or grpc must be configured to serve the sampling document")
}

func TestUnmarshalUsesDefaults(t *testing.T) {
	tcs := []struct {
		cfg      string
		expected jaeger_remote_sampling.Arguments
	}{
		// defaults http as expected
		{
			cfg: `
				http {}
				source {
					file = "remote.json"
				}
			`,
			expected: jaeger_remote_sampling.Arguments{
				HTTP: &jaeger_remote_sampling.HTTPServerArguments{
					Endpoint:              "0.0.0.0:5778",
					CompressionAlgorithms: otelcol.DefaultCompressionAlgorithms,
				},
				Source: jaeger_remote_sampling.ArgumentsSource{File: "remote.json"},
				DebugMetrics: otelcolCfg.DebugMetricsArguments{
					DisableHighCardinalityMetrics: true,
					Level:                         otelcolCfg.LevelDetailed,
				},
			},
		},
		// defaults grpc as expected
		{
			cfg: `
				grpc {}
				source {
					file = "remote.json"
				}
			`,
			expected: jaeger_remote_sampling.Arguments{
				GRPC:   &jaeger_remote_sampling.GRPCServerArguments{Endpoint: "0.0.0.0:14250", Transport: "tcp"},
				Source: jaeger_remote_sampling.ArgumentsSource{File: "remote.json"},
				DebugMetrics: otelcolCfg.DebugMetricsArguments{
					DisableHighCardinalityMetrics: true,
					Level:                         otelcolCfg.LevelDetailed,
				},
			},
		},
		// leaves specified values on http
		{
			cfg: `
				http {
					endpoint = "blerg"
				}
				source {
					file = "remote.json"
				}
			`,
			expected: jaeger_remote_sampling.Arguments{
				HTTP: &jaeger_remote_sampling.HTTPServerArguments{
					Endpoint:              "blerg",
					CompressionAlgorithms: otelcol.DefaultCompressionAlgorithms,
				},
				Source: jaeger_remote_sampling.ArgumentsSource{File: "remote.json"},
				DebugMetrics: otelcolCfg.DebugMetricsArguments{
					DisableHighCardinalityMetrics: true,
					Level:                         otelcolCfg.LevelDetailed,
				},
			},
		},
		// leaves specified values on grpc
		{
			cfg: `
				grpc {
					endpoint = "blerg"
					transport = "blarg"
				}
				source {
					file = "remote.json"
				}
			`,
			expected: jaeger_remote_sampling.Arguments{
				GRPC:   &jaeger_remote_sampling.GRPCServerArguments{Endpoint: "blerg", Transport: "blarg"},
				Source: jaeger_remote_sampling.ArgumentsSource{File: "remote.json"},
				DebugMetrics: otelcolCfg.DebugMetricsArguments{
					DisableHighCardinalityMetrics: true,
					Level:                         otelcolCfg.LevelDetailed,
				},
			},
		},
		// tests source grpc defaults
		{
			cfg: `
				grpc {
					endpoint = "blerg"
					transport = "blarg"
				}
				source {
					remote {
						endpoint = "TestRemoteEndpoint"
					}
				}
			`,
			expected: jaeger_remote_sampling.Arguments{
				GRPC: &jaeger_remote_sampling.GRPCServerArguments{Endpoint: "blerg", Transport: "blarg"},
				Source: jaeger_remote_sampling.ArgumentsSource{
					Remote: &jaeger_remote_sampling.GRPCClientArguments{
						Endpoint:        "TestRemoteEndpoint",
						Headers:         map[string]string{},
						Compression:     otelcol.CompressionTypeGzip,
						WriteBufferSize: 512 * 1024,
						BalancerName:    otelcol.DefaultBalancerName,
					},
				},
				DebugMetrics: otelcolCfg.DebugMetricsArguments{
					DisableHighCardinalityMetrics: true,
					Level:                         otelcolCfg.LevelDetailed,
				},
			},
		},
	}

	for _, tc := range tcs {
		var args jaeger_remote_sampling.Arguments
		err := syntax.Unmarshal([]byte(tc.cfg), &args)
		require.NoError(t, err)
		require.Equal(t, tc.expected, args)
	}
}

func TestUnmarshalRequiresExactlyOneSource(t *testing.T) {
	tcs := []struct {
		cfg           string
		expectedError string
	}{
		{
			cfg: `
				http {}
				source {}
			`,
			expectedError: "one of contents, file or remote must be configured",
		},
		{
			cfg: `
				http {}
				source {
					file = "remote.json"
					remote {
						endpoint = "http://localhost:1234"
					}
				}
			`,
			expectedError: "only one of contents, file or remote can be configured",
		},
	}

	for _, tc := range tcs {
		var args jaeger_remote_sampling.Arguments
		err := syntax.Unmarshal([]byte(tc.cfg), &args)
		require.EqualError(t, err, tc.expectedError)
	}
}

func getFreeAddr(t *testing.T) string {
	t.Helper()

	portNumber, err := freeport.GetFreePort()
	require.NoError(t, err)

	return fmt.Sprintf("localhost:%d", portNumber)
}
