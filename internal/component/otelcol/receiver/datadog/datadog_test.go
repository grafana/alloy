package datadog_test

import (
	"fmt"
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/datadog"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
)

func TestRun(t *testing.T) {
	httpAddr := componenttest.GetFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.datadog")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		endpoint = "%s"

		output { /* no-op */ }
	`, httpAddr)

	var args datadog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		httpAddr := componenttest.GetFreeAddr(t)
		in := fmt.Sprintf(`
		endpoint = "%s"
		cors {
			allowed_origins = ["https://*.test.com", "https://test.com"]
		}

		read_timeout = "1h"

		debug_metrics {
			disable_high_cardinality_metrics = true
		}

		output { /* no-op */ }
		`, httpAddr)

		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		require.Equal(t, args.DebugMetricsConfig().DisableHighCardinalityMetrics, true)
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs, ok := (ext).(*datadogreceiver.Config)

		require.True(t, ok)

		require.Equal(t, otelArgs.Endpoint, httpAddr)
		require.Equal(t, len(otelArgs.CORS.Get().AllowedOrigins), 2)
		require.Equal(t, otelArgs.CORS.Get().AllowedOrigins[0], "https://*.test.com")
		require.Equal(t, otelArgs.CORS.Get().AllowedOrigins[1], "https://test.com")
		require.Equal(t, otelArgs.ReadTimeout, time.Hour)
	})

	t.Run("trace_id_cache_size", func(t *testing.T) {
		in := `
		trace_id_cache_size = 500

		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs := ext.(*datadogreceiver.Config)
		require.Equal(t, 500, otelArgs.TraceIDCacheSize)
	})

	t.Run("intake_proxy", func(t *testing.T) {
		in := `
		intake {
			behavior = "proxy"
			proxy {
				api {
					key  = "my-secret-key"
					site = "datadoghq.eu"
					fail_on_invalid_key = true
				}
			}
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs := ext.(*datadogreceiver.Config)

		require.Equal(t, "proxy", otelArgs.Intake.Behavior)
		require.Equal(t, configopaque.String("my-secret-key"), otelArgs.Intake.Proxy.API.Key)
		require.Equal(t, "datadoghq.eu", otelArgs.Intake.Proxy.API.Site)
		require.True(t, otelArgs.Intake.Proxy.API.FailOnInvalidKey)
	})

	t.Run("intake_proxy_default_site", func(t *testing.T) {
		in := `
		intake {
			behavior = "proxy"
			proxy {
				api {
					key = "my-secret-key"
				}
			}
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs := ext.(*datadogreceiver.Config)

		require.Equal(t, "proxy", otelArgs.Intake.Behavior)
		require.Equal(t, configopaque.String("my-secret-key"), otelArgs.Intake.Proxy.API.Key)
		require.Equal(t, "datadoghq.com", otelArgs.Intake.Proxy.API.Site)
	})

	t.Run("intake_disable", func(t *testing.T) {
		in := `
		intake {
			behavior = "disable"
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs := ext.(*datadogreceiver.Config)

		require.Equal(t, "disable", otelArgs.Intake.Behavior)
	})

	t.Run("intake_disable_ignores_proxy_block", func(t *testing.T) {
		in := `
		intake {
			behavior = "disable"
			proxy {
				api {
					key  = "my-secret-key"
					site = "datadoghq.eu"
				}
			}
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs := ext.(*datadogreceiver.Config)

		require.Equal(t, "disable", otelArgs.Intake.Behavior)
		require.Equal(t, configopaque.String(""), otelArgs.Intake.Proxy.API.Key)
		require.Equal(t, "", otelArgs.Intake.Proxy.API.Site)
		require.False(t, otelArgs.Intake.Proxy.API.FailOnInvalidKey)
	})

	t.Run("no_intake", func(t *testing.T) {
		in := `
		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs := ext.(*datadogreceiver.Config)

		require.Equal(t, "", otelArgs.Intake.Behavior)
	})
}

func TestArguments_Validate(t *testing.T) {
	// syntax.Unmarshal calls Validate() automatically, so validation errors
	// surface at unmarshal time.

	t.Run("invalid_intake_behavior", func(t *testing.T) {
		in := `
		intake {
			behavior = "bogus"
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		err := syntax.Unmarshal([]byte(in), &args)
		require.ErrorContains(t, err, `invalid value "bogus"`)
	})

	t.Run("proxy_behavior_without_proxy_block", func(t *testing.T) {
		in := `
		intake {
			behavior = "proxy"
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		err := syntax.Unmarshal([]byte(in), &args)
		require.ErrorContains(t, err, `proxy block with an api block is required`)
	})

	t.Run("valid_proxy_config", func(t *testing.T) {
		in := `
		intake {
			behavior = "proxy"
			proxy {
				api {
					key = "my-secret-key"
				}
			}
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
	})

	t.Run("valid_disable", func(t *testing.T) {
		in := `
		intake {
			behavior = "disable"
		}

		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
	})

	t.Run("valid_no_intake", func(t *testing.T) {
		in := `
		output { /* no-op */ }
		`
		var args datadog.Arguments
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
	})
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
			var args datadog.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
