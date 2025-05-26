package prometheusconvert_test

import (
	"testing"

	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/test_common"
	_ "github.com/grafana/alloy/internal/static/metrics/instance"
)

func TestConvert(t *testing.T) {
	test_common.TestDirectory(t, "testdata", ".yaml", true, []string{}, map[string]struct{}{}, prometheusconvert.Convert, false)
}
