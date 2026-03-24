//go:build linux || darwin

package staticconvert_test

import (
	"flag"
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/staticconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
	_ "github.com/grafana/alloy/internal/static/metrics/instance" // Imported to override default values via the init function.
)

// Set this flag to update snapshots e.g. `go test -v ./internal/converter/internal/staticconvert/ -fix-tests`
var fixTestsFlag = flag.Bool("fix-tests", false, "update the test files with the current generated content")

func TestConvert(t *testing.T) {
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{"-config.expand-env"}, map[string]struct{}{}, staticconvert.Convert, *fixTestsFlag)
	test_common.TestDirectory(t, "testdata-v2", ".yaml", true, []string{"-enable-features", "integrations-next", "-config.expand-env"},
		map[string]struct{}{}, staticconvert.Convert, *fixTestsFlag)
	test_common.TestDirectory(t, "testdata_linux", ".yaml", true, []string{"-config.expand-env"}, map[string]struct{}{}, staticconvert.Convert, *fixTestsFlag)
}
