package prometheusconvert_test

import (
	"testing"

	"github.com/grafana/agent/internal/converter/internal/prometheusconvert"
	"github.com/grafana/agent/internal/converter/internal/test_common"
	_ "github.com/grafana/agent/internal/static/metrics/instance"
)

func TestConvert(t *testing.T) {
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{}, prometheusconvert.Convert)
}
