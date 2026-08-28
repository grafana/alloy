package text_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/encoding/text"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
	"github.com/stretchr/testify/require"
)

func TestArguments_SetToDefault(t *testing.T) {
	defaultConfig := textencodingextension.NewFactory().CreateDefaultConfig().(*textencodingextension.Config)

	var args text.Arguments
	args.SetToDefault()

	require.Equal(t, defaultConfig.Encoding, args.Encoding)
	require.Equal(t, defaultConfig.MarshalingSeparator, args.MarshalingSeparator)
	require.Equal(t, defaultConfig.UnmarshalingSeparator, args.UnmarshalingSeparator)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	var args text.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		encoding = "utf-16"
		marshaling_separator = " | "
		unmarshaling_separator = "[|]+"
	`), &args))

	converted, err := args.Convert(component.Options{})
	require.NoError(t, err)
	require.Equal(t, &textencodingextension.Config{
		Encoding:              "utf-16",
		MarshalingSeparator:   " | ",
		UnmarshalingSeparator: "[|]+",
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
				encoding = "utf-16"
				marshaling_separator = " | "
				unmarshaling_separator = "[|]+"
			`,
		},
		{
			name: "unknown encoding",
			config: `
				encoding = "not-an-encoding"
			`,
			wantErr: "unsupported encoding",
		},
		{
			name: "malformed unmarshaling separator",
			config: `
				unmarshaling_separator = "["
			`,
			wantErr: "error parsing regexp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args text.Arguments
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

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "otelcol.encoding.text")
	require.NoError(t, err)

	var args text.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		encoding = "utf8"
		marshaling_separator = " / "
		unmarshaling_separator = "[|]+"
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
	decoderFactory, ok := exports.Handler.Extension.(encoding.LogsDecoderExtension)
	require.True(t, ok, "extension does not implement encoding.LogsDecoderExtension")

	logs, err := unmarshaler.UnmarshalLogs([]byte("first||second||"))
	require.NoError(t, err)
	require.Equal(t, 2, logs.LogRecordCount())
	require.Equal(t, "first", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	require.Equal(t, "second", logs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())

	marshaled, err := marshaler.MarshalLogs(logs)
	require.NoError(t, err)
	require.Equal(t, "first / second", string(marshaled))

	decoder, err := decoderFactory.NewLogsDecoder(strings.NewReader("third||fourth"), encoding.WithFlushItems(1))
	require.NoError(t, err)
	decoded, err := decoder.DecodeLogs()
	require.NoError(t, err)
	require.Equal(t, 1, decoded.LogRecordCount())
	require.Equal(t, "third", decoded.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
}
