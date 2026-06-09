//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component/otelcol"
)

// TestEmittedYAMLGolden locks the full emitted Beyla YAML for a maximally-populated
// config. It guards mechanical refactors of the translation (e.g. the move to
// Convert() methods): the output must stay byte-identical. Regenerate with
// UPDATE_GOLDEN=1 only when an intended translation change is made.
func TestEmittedYAMLGolden(t *testing.T) {
	var args Arguments
	fillValue(reflect.ValueOf(&args).Elem(), 0)
	args.Output = &otelcol.ConsumerArguments{
		Metrics: []otelcol.Consumer{nil},
		Traces:  []otelcol.Consumer{nil},
	}

	rt := Runtime{Port: 12345, HealthAddr: "@beyla-health", OTLPAddr: "@beyla-otlp"}
	got, err := yaml.Marshal(Build(args, rt, slog.New(slog.NewTextHandler(io.Discard, nil))))
	require.NoError(t, err)

	const golden = "testdata/beyla_full_config.golden.yaml"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, got, 0o644))
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
}
