package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
)

func TestExceptionsConfig(t *testing.T) {
	type uncheckedArguments Arguments
	var decoded uncheckedArguments
	require.NoError(t, syntax.Unmarshal([]byte(`
 exceptions {
  enabled = true
  go {
   panics = false
   formatted_errors = true
  }
 }
 discovery {
  instrument {
   exe_path = "*/dogfood"
  }
 }
 `), &decoded))
	args := Arguments(decoded)
	require.ErrorContains(t, args.Validate(), "exceptions require output.logs")
	args.Output = &otelcol.ConsumerArguments{Logs: []otelcol.Consumer{nil}}
	require.NoError(t, args.Validate())
	cfg := args.Convert(Runtime{OTLPAddr: "@test"})
	require.Equal(t, map[string]any{"enabled": true, "go": map[string]any{"panics": false, "formatted_errors": true}}, cfg["exceptions"])
	require.Equal(t, map[string]any{"endpoint": "unix://@test", "protocol": "http/protobuf"}, cfg["otel_logs_export"])
	require.NotContains(t, Arguments{}.Convert(Runtime{}), "exceptions")
	require.NotContains(t, Arguments{}.Convert(Runtime{}), "otel_logs_export")
}
