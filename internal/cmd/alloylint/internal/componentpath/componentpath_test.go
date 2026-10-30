package componentpath_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/grafana/alloy/internal/cmd/alloylint/internal/componentpath"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), componentpath.Analyzer, "github.com/grafana/alloy/internal/component/...")
}
