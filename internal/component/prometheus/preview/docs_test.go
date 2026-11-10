package preview

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	_ "github.com/grafana/alloy/internal/component/prometheus/relabel"
	_ "github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	_ "github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

// Keep the copyable pipeline and its component arguments checked against source,
// including the receiver capsule types. This doesn't contact Grafana Cloud.
func TestDocumentedConfiguration(t *testing.T) {
	reg, ok := component.Get("prometheus.preview")
	require.True(t, ok)
	require.Equal(t, featuregate.StabilityGenerallyAvailable, reg.Stability)

	text, err := os.ReadFile("../../../../docs/sources/reference/components/prometheus/prometheus.preview.md")
	require.NoError(t, err)
	require.Contains(t, string(text), "stage: general-availability")
	require.NotContains(t, string(text), "--stability.level")
	t.Setenv("GRAFANA_CLOUD_PROMETHEUS_URL", "https://example.com/api/prom/push")
	t.Setenv("GRAFANA_CLOUD_METRICS_USER", "12345")
	t.Setenv("GRAFANA_CLOUD_API_KEY", "test-only")
	t.Setenv("ALLOY_INSTANCE_ID", "test")

	receiver := Exports{Receiver: appenders.Noop{}}
	scope := vm.NewScope(map[string]any{
		"prometheus": map[string]any{
			"relabel":      map[string]any{"candidate": receiver, "payments": receiver},
			"preview":      map[string]any{"candidate": receiver, "payments": receiver, "payments_filtered": receiver},
			"remote_write": map[string]any{"reports": receiver, "production": receiver},
		},
	})
	blocks := regexp.MustCompile("(?s)```alloy\\n(.*?)```").FindAllSubmatch(text, -1)
	checked := 0
	for _, code := range blocks {
		if strings.Contains(string(code[1]), "<LABEL>") {
			continue
		}
		file, err := parser.ParseFile("docs-example.alloy", code[1])
		require.NoError(t, err)
		for _, stmt := range file.Body {
			block, ok := stmt.(*ast.BlockStmt)
			if !ok {
				continue // Standalone forward_to examples aren't component definitions.
			}
			reg, ok := component.Get(strings.Join(block.Name, "."))
			require.True(t, ok)
			args := reg.CloneArguments()
			require.NoError(t, vm.New(block.Body).Evaluate(scope, args))
			checked++
		}
	}
	require.Equal(t, 15, checked, "expected to validate the standalone pipeline and every stage of the worked example")
}
