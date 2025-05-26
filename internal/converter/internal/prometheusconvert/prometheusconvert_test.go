package prometheusconvert_test

import (
	"flag"
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
	_ "github.com/grafana/alloy/internal/static/metrics/instance"
)

// Set this flag to update snapshots e.g. `go test -v ./interal/converter/internal/prometheusconvert/...` -fix-tests
var fixTestsFlag = flag.Bool("fix-tests", false, "update the test files with the current generated content")

func TestConvert(t *testing.T) {
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{}, map[string]struct{}{}, prometheusconvert.Convert, *fixTestsFlag)
}
