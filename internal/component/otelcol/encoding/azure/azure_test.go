package azure_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/encoding/azure"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"
	"github.com/stretchr/testify/require"
)

func TestArguments_SetToDefault(t *testing.T) {
	defaultConfig := azureencodingextension.NewFactory().CreateDefaultConfig().(*azureencodingextension.Config)

	var args azure.Arguments
	args.SetToDefault()

	require.Equal(t, azure.ArgumentsFromConfig(defaultConfig), args)
	require.NotEmpty(t, args.Logs.TimeFormats)
	require.Equal(t, defaultConfig.Logs.TimeFormats, args.Logs.TimeFormats)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	var args azure.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		logs {
			time_formats       = ["2006-01-02 15:04:05"]
			include_categories = ["FrontDoorAccessLog", "AzureCdnAccessLog"]
			exclude_categories = ["AzureCdnAccessLog"]
		}

		metrics {
			time_formats = ["01/02/2006 15:04:05"]
			aggregations = ["count", "total"]
		}

		traces {
			time_formats = ["2006-01-02T15:04:05Z"]
		}
	`), &args))

	converted, err := args.Convert(component.Options{})
	require.NoError(t, err)
	cfg, ok := converted.(*azureencodingextension.Config)
	require.True(t, ok)

	require.Equal(t, []string{"2006-01-02 15:04:05"}, cfg.Logs.TimeFormats)
	require.Equal(t, []string{"FrontDoorAccessLog", "AzureCdnAccessLog"}, cfg.Logs.IncludeCategories)
	require.Equal(t, []string{"AzureCdnAccessLog"}, cfg.Logs.ExcludeCategories)
	require.Equal(t, []string{"01/02/2006 15:04:05"}, cfg.Metrics.TimeFormats)
	require.Equal(t, []string{"count", "total"}, stringsOf(cfg.Metrics.Aggregations))
	require.Equal(t, []string{"2006-01-02T15:04:05Z"}, cfg.Traces.TimeFormats)

	// Converting back must produce the same arguments.
	require.Equal(t, args, azure.ArgumentsFromConfig(cfg))
}

func TestArguments_UnmarshalAlloy_EmptyBlocksKeepDefaults(t *testing.T) {
	defaultConfig := azureencodingextension.NewFactory().CreateDefaultConfig().(*azureencodingextension.Config)

	var args azure.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		logs {}
		metrics {}
		traces {}
	`), &args))

	converted, err := args.Convert(component.Options{})
	require.NoError(t, err)
	require.Equal(t, defaultConfig, converted)
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:   "empty config",
			config: ``,
		},
		{
			name: "all supported aggregations",
			config: `
				metrics {
					aggregations = ["count", "total", "minimum", "maximum", "average"]
				}
			`,
		},
		{
			name: "unknown aggregation",
			config: `
				metrics {
					aggregations = ["count", "unknown"]
				}
			`,
			wantErr: `invalid aggregation "unknown"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args azure.Arguments
			err := syntax.Unmarshal([]byte(tt.config), &args)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestExtension(t *testing.T) {
	ctx, cancel := context.WithTimeout(componenttest.TestContext(t), 10*time.Second)
	runErr := make(chan error, 1)

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "otelcol.encoding.azure")
	require.NoError(t, err)

	var args azure.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		logs {
			exclude_categories = ["AuditEvent"]
		}

		metrics {
			aggregations = ["count"]
		}
	`), &args))

	go func() {
		runErr <- ctrl.Run(ctx, args)
	}()
	defer func() {
		cancel()
		select {
		case err := <-runErr:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			require.Fail(t, "component did not stop")
		}
	}()

	require.NoError(t, ctrl.WaitRunning(5*time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(5*time.Second), "component never exported anything")
	startedComponent, err := ctrl.GetComponent()
	require.NoError(t, err, "no component added in controller")
	healthComponent, ok := startedComponent.(component.HealthComponent)
	require.True(t, ok, "component does not implement component.HealthComponent")
	require.Eventually(t, func() bool {
		return healthComponent.CurrentHealth().Health == component.HealthTypeHealthy
	}, 5*time.Second, 10*time.Millisecond, "timed out waiting for the component to be healthy")

	exports, ok := ctrl.Exports().(extension.Exports)
	require.True(t, ok)
	require.NotNil(t, exports.Handler)
	require.NotNil(t, exports.Handler.Extension)

	logsUnmarshaler, ok := exports.Handler.Extension.(encoding.LogsUnmarshalerExtension)
	require.True(t, ok, "extension does not implement encoding.LogsUnmarshalerExtension")
	metricsUnmarshaler, ok := exports.Handler.Extension.(encoding.MetricsUnmarshalerExtension)
	require.True(t, ok, "extension does not implement encoding.MetricsUnmarshalerExtension")
	_, ok = exports.Handler.Extension.(encoding.TracesUnmarshalerExtension)
	require.True(t, ok, "extension does not implement encoding.TracesUnmarshalerExtension")

	// The AuditEvent record is dropped by exclude_categories.
	logs, err := logsUnmarshaler.UnmarshalLogs([]byte(`{
		"records": [
			{
				"time": "2022-11-11T04:48:27.6767145Z",
				"resourceId": "/RESOURCE_ID",
				"operationName": "SecretGet",
				"category": "AuditEvent"
			},
			{
				"time": "2022-11-11T04:48:27.6767145Z",
				"resourceId": "/RESOURCE_ID",
				"operationName": "SecretList",
				"category": "OtherEvent"
			}
		]
	}`))
	require.NoError(t, err)
	require.Equal(t, 1, logs.LogRecordCount())
	category, ok := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("azure.category")
	require.True(t, ok)
	require.Equal(t, "OtherEvent", category.Str())

	// Only the count aggregation is emitted for each Azure metric.
	metrics, err := metricsUnmarshaler.UnmarshalMetrics([]byte(`{
		"records": [
			{
				"count": 2,
				"total": 0.217,
				"minimum": 0.042,
				"maximum": 0.175,
				"average": 0.1085,
				"resourceId": "/RESOURCE_ID",
				"time": "2023-04-18T09:03:00.0000000Z",
				"metricName": "CpuTime",
				"timeGrain": "PT1M"
			}
		]
	}`))
	require.NoError(t, err)
	require.Equal(t, 1, metrics.MetricCount())
	require.Equal(t, "cputime_count", metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())
}

func stringsOf[T ~string](in []T) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, string(v))
	}
	return out
}
