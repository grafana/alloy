//go:build !freebsd

package otelcolconvert_test

import (
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
)

var diagsToIgnore map[string]struct{}

func init() {
	// These diagnostics are expected and we only want to check them in TestConvertTelemetry.
	// If we check them in every test we'd have to create too many ".diag" files.
	diagsToIgnore = map[string]struct{}{
		"(Warning) the service/telemetry/logs/sampling configuration is not supported":                                                                                                             {},
		"(Warning) the service/telemetry/metrics/readers configuration is not supported - to gather Alloy's own telemetry refer to: https://grafana.com/docs/alloy/latest/collect/metamonitoring/": {},
	}
}

func TestConvert(t *testing.T) {
	// TODO(rfratto): support -update flag.
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.Convert)
	// test_common.TestDirectory(t, "testdata/otelcol_dedup", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.Convert)
	// test_common.TestDirectory(t, "testdata/otelcol_without_validation", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.ConvertWithoutValidation)
}

// TestConvertErrors tests errors specifically regarding the reading of
// OpenTelemetry configurations.
func TestConvertErrors(t *testing.T) {
	test_common.TestDirectory(t, "testdata/otelcol_errors", ".yaml", true, []string{},
		diagsToIgnore, otelcolconvert.Convert)
}

func TestConvertTelemetry(t *testing.T) {
	test_common.TestDirectory(t, "testdata/otelcol_telemetry", ".yaml", true, []string{},
		map[string]struct{}{}, otelcolconvert.Convert)
}

func TestConvertEnvvars(t *testing.T) {
	test_common.TestDirectory(t, "testdata/envvars", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.Convert)
}
