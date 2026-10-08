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

// benchOutput renders count runs of go test -bench output for pkg, with one
// line per benchmark. Each benchmark is given as ns/op and allocs/op; ns/op
// jitters slightly between runs.
func benchOutput(pkg string, count int, benches map[string][2]float64) string {
	var sb strings.Builder
	for i := range count {
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

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestRenderReport(t *testing.T) {
	const pkg = "github.com/grafana/alloy/internal/component/loki/process/stages"

	base := benchOutput(pkg, 10, map[string][2]float64{
		"Match/all":     {1000, 300},
		"Regex":         {2000, 19},
		"Template":      {3000, 40},
		"SplitJSON":     {5000, 400},
		"Removed/thing": {100, 1},
	})
	head := benchOutput(pkg, 10, map[string][2]float64{
		"Match/all":   {1000, 450}, // more allocations only
		"Regex":       {1500, 19},  // faster
		"Template":    {3030, 40},  // +1%: below threshold
		"SplitJSON":   {6000, 400}, // slower
		"Added/thing": {100, 1},
	})

	baseRes, err := readResults(writeTemp(t, "base.txt", base))
	require.NoError(t, err)
	headRes, err := readResults(writeTemp(t, "head.txt", head))
	require.NoError(t, err)

	report := renderReport(reportInput{
		base: baseRes,
		head: headRes,
		selection: Selection{Packages: []Package{
			{ImportPath: pkg, Reason: ReasonDirect, InBase: true, InHead: true},
		}},
		meta: Meta{Count: 10, Failures: []Failure{
			{ImportPath: "github.com/grafana/alloy/internal/broken", Tree: TreeHead, Stage: "build", Message: "undefined: foo"},
		}},
		benchstat: "benchstat output",
		mergeBase: "0123456789abcdef",
		headSHA:   "fedcba9876543210",
		runner:    "ubuntu-x64-xlarge",
		threshold: 5,
		alpha:     0.05,
		maxRows:   30,
	})

	require.True(t, strings.HasPrefix(report, Marker))
	require.Contains(t, report, "merge base `0123456789` with PR head `fedcba9876`, 10 interleaved runs each on `ubuntu-x64-xlarge` (Test CPU @ 3.00GHz)")
	require.Contains(t, report, "Benchmarked 1 package (1 changed directly)")

	cpu, allocs := section(report, "### CPU time"), section(report, "### Allocations")
	require.Contains(t, cpu, "2 benchmarks changed significantly out of 4 compared")
	require.Contains(t, cpu, "| 🔴 | `internal/component/loki/process/stages` | `SplitJSON-16` | 5.000µ | 6.000µ | +20.00% |")
	require.Contains(t, cpu, "| 🟢 | `internal/component/loki/process/stages` | `Regex-16` | 2.000µ | 1.500µ | -25.00% |")
	require.Less(t, strings.Index(cpu, "SplitJSON"), strings.Index(cpu, "Regex"), "regressions come first")
	require.NotContains(t, cpu, "Template")
	require.NotContains(t, cpu, "Match")

	require.Contains(t, allocs, "1 benchmark changed significantly out of 4 compared")
	require.Contains(t, allocs, "| 🔴 | `internal/component/loki/process/stages` | `Match/all-16` | 300.0 | 450.0 | +50.00% |")

	require.Contains(t, section(report, "### Only in base"), "`Removed/thing-16`")
	require.Contains(t, section(report, "### Only in PR"), "`Added/thing-16`")
	require.Contains(t, section(report, "### ⚠️ Failures"), "`internal/broken` failed to build on head")
	require.Contains(t, report, "<details><summary>Full benchstat output</summary>\n\n```\nbenchstat output\n```\n</details>")
}

func TestRenderReport_NoChanges(t *testing.T) {
	const pkg = "github.com/grafana/alloy/syntax/vm"
	out := benchOutput(pkg, 10, map[string][2]float64{"Eval": {1000, 10}})

	res, err := readResults(writeTemp(t, "out.txt", out))
	require.NoError(t, err)

	report := renderReport(reportInput{
		base: res, head: res,
		selection: Selection{Packages: []Package{{ImportPath: pkg, Reason: ReasonReverseDep}}},
		threshold: 5, alpha: 0.05, maxRows: 30,
	})
	require.Contains(t, report, "Benchmarked 1 package (1 importing changed packages)")
	require.Contains(t, section(report, "### CPU time"), "No significant changes across 1 benchmark.")
	require.Contains(t, section(report, "### Allocations"), "No significant changes across 1 benchmark.")
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

// section returns the part of report starting at heading, up to the next
// heading.
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
