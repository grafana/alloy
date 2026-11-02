package benchdiff

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// benchOutput renders 10 runs of go test -bench output for benchmarks given as
// {ns/op, allocs/op}, with a little jitter in ns/op.
func benchOutput(pkg string, benches map[string][2]float64) string {
	var sb strings.Builder
	for i := range 10 {
		fmt.Fprintf(&sb, "goos: linux\ngoarch: amd64\npkg: %s\ncpu: Test CPU @ 3.00GHz\n", pkg)
		for _, name := range slices.Sorted(maps.Keys(benches)) {
			v := benches[name]
			ns := v[0] * (1 + float64(i%3-1)*0.001)
			fmt.Fprintf(&sb, "Benchmark%s-16\t1000\t%.1f ns/op\t%d B/op\t%d allocs/op\n", name, ns, int(v[1])*8, int(v[1]))
		}
		fmt.Fprintf(&sb, "PASS\nok\t%s\t1.0s\n", pkg)
	}
	return sb.String()
}

func writeTemp(t testing.TB, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestRenderReport(t *testing.T) {
	const pkg = "github.com/grafana/alloy/internal/component/loki/process/stages"

	base := benchOutput(pkg, map[string][2]float64{
		"Match/all":     {1000, 300},
		"Regex":         {2000, 19},
		"Template":      {3000, 40},
		"SplitJSON":     {5000, 400},
		"Both":          {1000, 10},
		"Removed/thing": {100, 1},
	})
	head := benchOutput(pkg, map[string][2]float64{
		"Match/all":   {1000, 450}, // more allocations only
		"Regex":       {1500, 19},  // faster
		"Template":    {3030, 40},  // +1%: below threshold
		"SplitJSON":   {6000, 400}, // slower
		"Both":        {2000, 20},  // slower and more allocations
		"Added/thing": {100, 1},
	})

	baseRes, err := readResults(writeTemp(t, "base.txt", base))
	require.NoError(t, err)
	headRes, err := readResults(writeTemp(t, "head.txt", head))
	require.NoError(t, err)

	report := renderReport(reportInput{
		base:     baseRes,
		head:     headRes,
		packages: []Package{{ImportPath: pkg, InBase: true, InHead: true}},
		failures: []Failure{
			{ImportPath: "github.com/grafana/alloy/internal/broken", Checkout: checkoutHead, Stage: "build", Message: "undefined: foo"},
		},
		count:     10,
		benchstat: "benchstat output",
		mergeBase: "0123456789abcdef",
		headSHA:   "fedcba9876543210",
	})

	require.True(t, strings.HasPrefix(report, Marker))
	require.Contains(t, report, "merge base `0123456789` with PR head `fedcba9876`, 10 interleaved runs each on Test CPU @ 3.00GHz. Benchmarked 1 package")

	changes := section(report, "### Significant changes")
	require.Contains(t, changes, "4 benchmarks changed significantly out of 5 compared")
	rows := []string{
		"| 🔴 | `internal/component/loki/process/stages` | `Both-16` | CPU (sec/op) | 1.000µ | 2.000µ | +100.00% |",
		"| 🔴 | `internal/component/loki/process/stages` | `Both-16` | Allocations (allocs/op) | 10.00 | 20.00 | +100.00% |",
		"| 🔴 | `internal/component/loki/process/stages` | `Match/all-16` | Allocations (allocs/op) | 300.0 | 450.0 | +50.00% |",
		"| 🔴 | `internal/component/loki/process/stages` | `SplitJSON-16` | CPU (sec/op) | 5.000µ | 6.000µ | +20.00% |",
		"| 🟢 | `internal/component/loki/process/stages` | `Regex-16` | CPU (sec/op) | 2.000µ | 1.500µ | -25.00% |",
	}
	last := -1
	for _, row := range rows {
		i := strings.Index(changes, row)
		require.Greater(t, i, last, "missing or out of order: %s", row)
		last = i
	}
	require.NotContains(t, changes, "Template")

	require.Contains(t, section(report, "### Only in base"), "`Removed/thing-16`")
	require.Contains(t, section(report, "### Only in PR"), "`Added/thing-16`")
	require.Contains(t, section(report, "### ⚠️ Failures"), "`internal/broken` failed to build on head")
	require.Contains(t, report, "<details><summary>Full benchstat output</summary>\n\n```\nbenchstat output\n```\n</details>")
}

func TestRenderReport_NoChanges(t *testing.T) {
	const pkg = "github.com/grafana/alloy/syntax/vm"
	out := benchOutput(pkg, map[string][2]float64{"Eval": {1000, 10}})

	res, err := readResults(writeTemp(t, "out.txt", out))
	require.NoError(t, err)

	report := renderReport(reportInput{
		base: res, head: res,
		packages: []Package{{ImportPath: pkg}},
		count:    10,
	})
	require.Contains(t, section(report, "### Significant changes"), "No significant changes in CPU or allocations across 1 benchmark.")
	require.NotContains(t, report, "Only in")
}

func TestRenderReport_NoPackages(t *testing.T) {
	report := renderReport(reportInput{})
	require.Contains(t, report, "No packages with benchmarks are affected by this pull request.")
}

func TestCode(t *testing.T) {
	require.Equal(t, "`a\\|b`", code("a|b"))
	require.Equal(t, "a`b", code("a`b"))
}

// section returns report from heading up to the next heading.
func section(report, heading string) string {
	i := strings.Index(report, heading)
	if i < 0 {
		return ""
	}
	rest := report[i+len(heading):]
	if j := strings.Index(rest, "\n### "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}
