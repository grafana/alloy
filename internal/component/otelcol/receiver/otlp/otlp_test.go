package otlp_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/otlp"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

// Test performs a basic integration test which runs the otelcol.receiver.otlp
// component and ensures that it can receive and forward data.
func Test(t *testing.T) {
	httpAddr := componenttest.GetFreeAddr(t)
	grpcAddr := componenttest.GetFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.otlp")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		grpc {
			endpoint = "%s"
		}

		http {
			endpoint = "%s"
		}

		output {
			// no-op: will be overridden by test code.
		}
	`, grpcAddr, httpAddr)

	require.NoError(t, err)

	var args otlp.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	// Override our settings so telemetry gets forwarded to the test channels.
	traceCh := make(chan ptrace.Traces, 1)
	profilesCh := make(chan pprofile.Profiles, 2)
	profilesErrCh := make(chan error, 2)
	args.Output = makeOutput(traceCh, profilesCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))

	sendProfiles := func(transport string, request func() error) {
		go func() {
			bo := backoff.New(ctx, backoff.Config{
				MinBackoff: 10 * time.Millisecond,
				MaxBackoff: 100 * time.Millisecond,
			})
			for bo.Ongoing() {
				if err := request(); err != nil {
					l.Error("failed to send profiles", "transport", transport, "err", err)
					bo.Wait()
					continue
				}

				profilesErrCh <- nil
				return
			}
			profilesErrCh <- bo.Err()
		}()
	}

	sendProfiles("gRPC", func() error {
		conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		defer conn.Close()

		client := pprofileotlp.NewGRPCClient(conn)
		_, err = client.Export(ctx, pprofileotlp.NewExportRequestFromProfiles(createTestProfiles()))
		return err
	})

	sendProfiles("HTTP", func() error {
		exportRequest := pprofileotlp.NewExportRequestFromProfiles(createTestProfiles())
		body, err := exportRequest.MarshalProto()
		if err != nil {
			return err
		}

		profilesURL := fmt.Sprintf("http://%s/v1development/profiles", httpAddr)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, profilesURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/x-protobuf")

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected profiles response status: %s", resp.Status)
		}
		return nil
	})

	// Send traces in the background to our receiver.
	go func() {
		request := func() error {
			f, err := os.Open("testdata/payload.json")
			require.NoError(t, err)
			defer f.Close()

			tracesURL := fmt.Sprintf("http://%s/v1/traces", httpAddr)
			_, err = http.DefaultClient.Post(tracesURL, "application/json", f)
			return err
		}

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			if err := request(); err != nil {
				l.Error("failed to send traces", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our client to get a span.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for traces")
	case tr := <-traceCh:
		require.Equal(t, 1, tr.SpanCount())
	}

	for range 2 {
		select {
		case <-time.After(time.Second):
			require.FailNow(t, "failed waiting for profiles request")
		case err := <-profilesErrCh:
			require.NoError(t, err)
		}
	}

	for range 2 {
		select {
		case <-time.After(time.Second):
			require.FailNow(t, "failed waiting for profiles")
		case profiles := <-profilesCh:
			require.Equal(t, 1, profiles.ProfileCount())
		}
	}
}

// makeOutput returns ConsumerArguments which forward telemetry to the provided channels.
func makeOutput(traceCh chan ptrace.Traces, profilesCh chan pprofile.Profiles) *otelcol.ConsumerArguments {
	traceConsumer := fakeconsumer.Consumer{
		ConsumeTracesFunc: func(ctx context.Context, t ptrace.Traces) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case traceCh <- t:
				return nil
			}
		},
	}
	profilesConsumer := fakeconsumer.Consumer{
		ConsumeProfilesFunc: func(ctx context.Context, profiles pprofile.Profiles) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case profilesCh <- profiles:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Traces:   []otelcol.Consumer{&traceConsumer},
		Profiles: []otelcol.Consumer{&profilesConsumer},
	}
}

func createTestProfiles() pprofile.Profiles {
	return testdata.GenerateProfiles(1)
}

func TestUnmarshalDefault(t *testing.T) {
	alloyCfg := `
		http {}
		grpc {}
		output {}
	`
	var args otlp.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	actual, err := args.Convert()
	require.NoError(t, err)

	expected := otlpreceiver.Config{
		Protocols: otlpreceiver.Protocols{
			GRPC: configoptional.Some[configgrpc.ServerConfig](configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  "0.0.0.0:4317",
					Transport: "tcp",
				},
				ReadBufferSize: 524288,
				Keepalive: configoptional.Some[configgrpc.KeepaliveServerConfig](configgrpc.KeepaliveServerConfig{
					ServerParameters:  configoptional.Some[configgrpc.KeepaliveServerParameters](configgrpc.KeepaliveServerParameters{}),
					EnforcementPolicy: configoptional.Some[configgrpc.KeepaliveEnforcementPolicy](configgrpc.KeepaliveEnforcementPolicy{}),
				}),
			}),
			HTTP: configoptional.Some[otlpreceiver.HTTPConfig](otlpreceiver.HTTPConfig{
				ServerConfig: confighttp.ServerConfig{
					NetAddr:               confignet.AddrConfig{Endpoint: "0.0.0.0:4318", Transport: confignet.TransportTypeTCP},
					CompressionAlgorithms: []string{"", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"},
					CORS:                  configoptional.Some[confighttp.CORSConfig](confighttp.CORSConfig{}),
					KeepAlivesEnabled:     true,
					IdleTimeout:           1 * time.Minute,
					ReadHeaderTimeout:     1 * time.Minute,
					WriteTimeout:          30 * time.Second,
				},
				TracesURLPath:  "/v1/traces",
				MetricsURLPath: "/v1/metrics",
				LogsURLPath:    "/v1/logs",
			}),
		},
	}

	require.Equal(t, &expected, actual)
}

func TestUnmarshalGrpc(t *testing.T) {
	alloyCfg := `
		grpc {
			endpoint = "/v1/traces"
		}

		output {
		}
	`
	var args otlp.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
}

func TestUnmarshalHttp(t *testing.T) {
	alloyCfg := `
		http {
			endpoint = "/v1/traces"
		}

		output {
		}
	`
	var args otlp.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	assert.Equal(t, "/v1/logs", args.HTTP.LogsURLPath)
	assert.Equal(t, "/v1/metrics", args.HTTP.MetricsURLPath)
	assert.Equal(t, "/v1/traces", args.HTTP.TracesURLPath)
}

func TestUnmarshalHttpUrls(t *testing.T) {
	alloyCfg := `
		http {
			endpoint = "/v1/traces"
			traces_url_path = "custom/traces"
			metrics_url_path = "custom/metrics"
			logs_url_path = "custom/logs"
		}

		output {
		}
	`
	var args otlp.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	assert.Equal(t, "custom/logs", args.HTTP.LogsURLPath)
	assert.Equal(t, "custom/metrics", args.HTTP.MetricsURLPath)
	assert.Equal(t, "custom/traces", args.HTTP.TracesURLPath)
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
			grpc {
				endpoint = "/v1/traces"
			}
			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			alloyCfg: `
			grpc {
				endpoint = "/v1/traces"
			}
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_true",
			alloyCfg: `
			grpc {
				endpoint = "/v1/traces"
			}
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args otlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
