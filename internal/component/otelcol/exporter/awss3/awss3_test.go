package awss3_test

import (
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/awss3"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"
	"github.com/stretchr/testify/require"
)

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			agentCfg: `
			s3_uploader {
				s3_bucket = "test"
				s3_prefix = "logs"
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
		{
			testName: "no_optional_debug",
			agentCfg: `
			s3_uploader {
				s3_bucket = "test"
				s3_prefix = "logs"
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			agentCfg: `
			s3_uploader {
				s3_bucket = "test"
				s3_prefix = "logs"
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
			agentCfg: `
			s3_uploader {
				s3_bucket = "test"
				s3_prefix = "logs"
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
		{
			testName: "explicit_debug_level",
			agentCfg: `
			s3_uploader {
				s3_bucket = "test"
				s3_prefix = "logs"
			}
			debug_metrics {
				level = "none"
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelNone,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args awss3.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}

// Checks that the component can start with the sumo_ic marshaler.
func TestSumoICMarshaler(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.awss3")
	require.NoError(t, err)

	cfg := `
		s3_uploader {
			s3_bucket = "test"
			s3_prefix = "logs"
		}

		marshaler {
			type = "sumo_ic"
		}
	`
	var args awss3.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
}

// Checks that the component can be updated with the sumo_ic marshaler.
func TestSumoICMarshalerUpdate(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.awss3")
	require.NoError(t, err)

	cfg := `
		s3_uploader {
			s3_bucket = "test"
			s3_prefix = "logs"
		}

		marshaler {
			type = "otlp_json"
		}
	`
	var args awss3.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")

	cfg2 := `
		s3_uploader {
			s3_bucket = "test"
			s3_prefix = "logs"
		}

		marshaler {
			type = "sumo_ic"
		}
	`

	var args2 awss3.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg2), &args2))
	require.NoError(t, ctrl.Update(args2))
}

func TestConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected awss3exporter.S3UploaderConfig
	}{
		{
			testName: "default",
			agentCfg: `
			s3_uploader {
				s3_bucket = "test"
				s3_prefix = "logs"
			}
			`,
			expected: awss3exporter.S3UploaderConfig{
				S3Bucket:          "test",
				S3Prefix:          "logs",
				S3PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:        "",
				Endpoint:          "",
				RoleArn:           "",
				S3ForcePathStyle:  false,
				DisableSSL:        false,
				Compression:       "none",
			},
		},
		{
			testName: "explicit_values",
			agentCfg: `
			s3_uploader {
				s3_bucket = "test"
				s3_prefix = "logs"
				s3_partition_format = "year=%Y/month=%m/day=%d/hour=%H/minute=%M"
				file_prefix = "prefix"
				endpoint = "https://s3.amazonaws.com"
				role_arn = "arn:aws:iam::123456789012:role/test"
				s3_force_path_style = true
				disable_ssl = true
				compression = "gzip"
			}
			`,
			expected: awss3exporter.S3UploaderConfig{
				S3Bucket:          "test",
				S3Prefix:          "logs",
				S3PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:        "prefix",
				Endpoint:          "https://s3.amazonaws.com",
				RoleArn:           "arn:aws:iam::123456789012:role/test",
				S3ForcePathStyle:  true,
				DisableSSL:        true,
				Compression:       "gzip",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args awss3.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.S3Uploader.Convert())
		})
	}
}
