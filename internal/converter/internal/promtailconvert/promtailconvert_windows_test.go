//go:build windows

package promtailconvert_test

import (
	"flag"
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/promtailconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
	_ "github.com/grafana/alloy/internal/static/metrics/instance" // Imported to override default values via the init function.
)

// Set this flag to update snapshots e.g. `go test -v ./interal/converter/internal/promtailconverter/...` -fix-tests
var fixTestsFlag = flag.Bool("fix-tests", false, "update the test files with the current generated content")

func TestConvert(t *testing.T) {
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{}, map[string]struct{}{}, promtailconvert.Convert, *fixTestsFlag)
	test_common.TestDirectory(t, "testdata_windows", ".yaml", true, []string{}, map[string]struct{}{}, promtailconvert.Convert, *fixTestsFlag)
}
