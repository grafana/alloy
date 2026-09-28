package sqlfingerprint_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	_ "github.com/grafana/alloy/internal/component/otelcol/exporter/otlp"
	_ "github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	_ "github.com/grafana/alloy/internal/component/otelcol/processor/sqlfingerprint"
	_ "github.com/grafana/alloy/internal/component/otelcol/receiver/otlp"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/validator"
)

// TestExampleConfiguration validates real component references and argument
// types in the runnable example, including the experimental stability gate.
func TestExampleConfiguration(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(file), "../../../../..", "example/sql-fingerprint/config.alloy")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	options := validator.Options{
		Sources:           map[string][]byte{path: content},
		ComponentRegistry: component.NewDefaultRegistry(featuregate.StabilityExperimental, false),
		MinStability:      featuregate.StabilityExperimental,
	}
	require.NoError(t, validator.Validate(options))
	options.MinStability = featuregate.StabilityGenerallyAvailable
	options.ComponentRegistry = component.NewDefaultRegistry(featuregate.StabilityGenerallyAvailable, false)
	require.ErrorContains(t, validator.Validate(options), "experimental")
}
