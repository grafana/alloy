//go:build !freebsd

package otelcolconvert_test

import (
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
)

func TestConvert(t *testing.T) {
	// TODO(rfratto): support -update flag.
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{}, otelcolconvert.Convert)
	test_common.TestDirectory(t, "testdata/otelcol_without_validation", ".yaml", true, []string{}, otelcolconvert.ConvertWithoutValidation)
}

// TestConvertErrors tests errors specifically regarding the reading of
// OpenTelemetry configurations.
func TestConvertErrors(t *testing.T) {
	test_common.TestDirectory(t, "testdata/otelcol_errors", ".yaml", true, []string{}, otelcolconvert.Convert)
}
