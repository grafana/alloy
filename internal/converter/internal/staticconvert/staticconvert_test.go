package staticconvert_test

import (
	"runtime"
	"testing"

	"github.com/grafana/agent/internal/converter/internal/staticconvert"
	"github.com/grafana/agent/internal/converter/internal/test_common"
	_ "github.com/grafana/agent/internal/static/metrics/instance" // Imported to override default values via the init function.
)

func TestConvert(t *testing.T) {
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{"-config.expand-env"}, staticconvert.Convert)
	test_common.TestDirectory(t, "testdata-v2", ".yaml", true, []string{"-enable-features", "integrations-next", "-config.expand-env"}, staticconvert.Convert)

	if runtime.GOOS == "windows" {
		test_common.TestDirectory(t, "testdata_windows", ".yaml", true, []string{}, staticconvert.Convert)
		test_common.TestDirectory(t, "testdata-v2_windows", ".yaml", true, []string{"-enable-features", "integrations-next"}, staticconvert.Convert)
	}
}
