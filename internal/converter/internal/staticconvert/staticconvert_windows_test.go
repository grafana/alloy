//go:build windows

package staticconvert_test

import (
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/staticconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
	_ "github.com/grafana/alloy/internal/static/metrics/instance" // Imported to override default values via the init function.
)

func TestConvert(t *testing.T) {
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{"-config.expand-env"}, map[string]struct{}{}, staticconvert.Convert)
	test_common.TestDirectory(t, "testdata-v2_windows", ".yaml", true, []string{"-enable-features", "integrations-next"}, map[string]struct{}{}, staticconvert.Convert)
	test_common.TestDirectory(t, "testdata_windows", ".yaml", true, []string{"-config.expand-env"}, map[string]struct{}{}, staticconvert.Convert)
}
