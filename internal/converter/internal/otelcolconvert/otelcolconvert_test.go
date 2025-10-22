//go:build !freebsd && !openbsd

package otelcolconvert_test

import (
	"flag"
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
)

// Set this flag to update snapshots e.g. `go test -v ./interal/converter/internal/otelcolconverter/...` -fix-tests
var fixTestsFlag = flag.Bool("fix-tests", false, "update the test files with the current generated content")

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
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.Convert, *fixTestsFlag)
	test_common.TestDirectory(t, "testdata/otelcol_dedup", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.Convert, *fixTestsFlag)
	test_common.TestDirectory(t, "testdata/otelcol_without_validation", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.ConvertWithoutValidation, *fixTestsFlag)
}

// TestConvertErrors tests errors specifically regarding the reading of
// OpenTelemetry configurations.
func TestConvertErrors(t *testing.T) {
	test_common.TestDirectory(t, "testdata/otelcol_errors", ".yaml", true, []string{},
		diagsToIgnore, otelcolconvert.Convert, *fixTestsFlag)
}

func TestConvertTelemetry(t *testing.T) {
	test_common.TestDirectory(t, "testdata/otelcol_telemetry", ".yaml", true, []string{},
		map[string]struct{}{}, otelcolconvert.Convert, *fixTestsFlag)
}

func TestConvertEnvvars(t *testing.T) {
	test_common.TestDirectory(t, "testdata/envvars", ".yaml", true, []string{}, diagsToIgnore, otelcolconvert.Convert, *fixTestsFlag)
}
