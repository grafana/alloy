package jsonlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/encoding/jsonlog"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"
	"github.com/stretchr/testify/require"
)

func TestArguments_SetToDefault(t *testing.T) {
	defaultConfig := jsonlogencodingextension.NewFactory().CreateDefaultConfig().(*jsonlogencodingextension.Config)

	var args jsonlog.Arguments
	args.SetToDefault()

	require.Equal(t, defaultConfig.ArrayMode, args.ArrayMode)
	require.Equal(t, string(defaultConfig.Mode), args.Mode)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	var args jsonlog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		array_mode = false
		mode = "body_with_inline_attributes"
	`), &args))

	converted, err := args.Convert(component.Options{})
	require.NoError(t, err)
	require.Equal(t, &jsonlogencodingextension.Config{
		ArrayMode: false,
		Mode:      jsonlogencodingextension.JSONEncodingModeBodyWithInlineAttributes,
	}, converted)
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "valid non-default config",
			config: `
				array_mode = false
				mode = "body_with_inline_attributes"
			`,
		},
		{
			name: "unsupported mode",
			config: `
				mode = "unsupported"
			`,
			wantErr: `invalid mode "unsupported"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args jsonlog.Arguments
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

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "otelcol.encoding.jsonlog")
	require.NoError(t, err)

	var args jsonlog.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		array_mode = false
		mode = "body"
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

	exports, ok := ctrl.Exports().(extension.Exports)
	require.True(t, ok)
	require.NotNil(t, exports.Handler)
	require.NotNil(t, exports.Handler.Extension)

	marshaler, ok := exports.Handler.Extension.(encoding.LogsMarshalerExtension)
	require.True(t, ok, "extension does not implement encoding.LogsMarshalerExtension")
	unmarshaler, ok := exports.Handler.Extension.(encoding.LogsUnmarshalerExtension)
	require.True(t, ok, "extension does not implement encoding.LogsUnmarshalerExtension")

	logs, err := unmarshaler.UnmarshalLogs([]byte("{\"message\":\"first\"}\n{\"message\":\"second\"}"))
	require.NoError(t, err)
	require.Equal(t, 2, logs.LogRecordCount())
	first, ok := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().Get("message")
	require.True(t, ok)
	require.Equal(t, "first", first.Str())
	second, ok := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Body().Map().Get("message")
	require.True(t, ok)
	require.Equal(t, "second", second.Str())

	marshaled, err := marshaler.MarshalLogs(logs)
	require.NoError(t, err)
	require.Equal(t, "{\"message\":\"first\"}\n{\"message\":\"second\"}", string(marshaled))

	roundTripped, err := unmarshaler.UnmarshalLogs(marshaled)
	require.NoError(t, err)
	require.Equal(t, logs.LogRecordCount(), roundTripped.LogRecordCount())
}
