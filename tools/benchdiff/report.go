package benchdiff

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchmath"
	"golang.org/x/perf/benchunit"
)

// Marker identifies the benchmark report comment on a pull request.
const Marker = "<!-- alloy-benchmark-report -->"

const (
	alpha     = 0.05 // significance level, the same as benchstat's
	threshold = 5.0  // minimum change in percent to report
	maxRows   = 30

	maxBenchstatLen = 30_000
	maxReportLen    = 60_000
)

var reportUnits = []struct{ unit, title string }{
	{"sec/op", "CPU time (sec/op)"},
	{"allocs/op", "Allocations (allocs/op)"},
}

type benchKey struct {
	pkg, name string
}

// results holds benchmark samples by benchmark and unit.
type results struct {
	cpu     string
	samples map[benchKey]map[string][]float64
}

func readResults(file string) (results, error) {
	res := results{samples: map[benchKey]map[string][]float64{}}

	f, err := os.Open(file)
	if err != nil {
		return res, err
	}
	defer f.Close()

	r := benchfmt.NewReader(f, file)
	for r.Scan() {
		rec, ok := r.Result().(*benchfmt.Result)
		if !ok {
			continue
		}
		if res.cpu == "" {
			res.cpu = rec.GetConfig("cpu")
		}
		key := benchKey{pkg: rec.GetConfig("pkg"), name: string(rec.Name.Full())}
		if res.samples[key] == nil {
			res.samples[key] = map[string][]float64{}
		}
		for _, v := range rec.Values {
			res.samples[key][v.Unit] = append(res.samples[key][v.Unit], v.Value)
		}
	}
	return res, r.Err()
}

type change struct {
	key        benchKey
	base, head float64
	delta      float64 // percent
}

// compareUnit returns the significant changes in unit, regressions first, each
// group ordered by size, and the number of benchmarks compared.
func compareUnit(base, head results, unit string) ([]change, int) {
	var changes []change
	compared := 0
	for key, baseUnits := range base.samples {
		baseValues, headValues := baseUnits[unit], head.samples[key][unit]
		if len(baseValues) == 0 || len(headValues) == 0 {
			continue
		}
		compared++

		bs := benchmath.NewSample(baseValues, &benchmath.DefaultThresholds)
		hs := benchmath.NewSample(headValues, &benchmath.DefaultThresholds)
		if benchmath.AssumeNothing.Compare(bs, hs).P >= alpha {
			continue
		}

		c := change{
			key:  key,
			base: benchmath.AssumeNothing.Summary(bs, 0.95).Center,
			head: benchmath.AssumeNothing.Summary(hs, 0.95).Center,
		}
		switch c.base {
		case c.head:
			continue
		case 0:
			c.delta = math.Inf(1)
		default:
			c.delta = (c.head - c.base) / c.base * 100
		}
		if math.Abs(c.delta) >= threshold {
			changes = append(changes, c)
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if (a.delta > 0) != (b.delta > 0) {
			return a.delta > 0
		}
		if math.Abs(a.delta) != math.Abs(b.delta) {
			return math.Abs(a.delta) > math.Abs(b.delta)
		}
		return lessKey(a.key, b.key)
	})
	return changes, compared
}

// onlyIn returns the benchmarks in a that aren't in b.
func onlyIn(a, b results) []benchKey {
	var keys []benchKey
	for key := range a.samples {
		if _, ok := b.samples[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })
	return keys
}

func lessKey(a, b benchKey) bool {
	if a.pkg != b.pkg {
		return a.pkg < b.pkg
	}
	return a.name < b.name
}

type reportInput struct {
	base, head         results
	packages           []Package
	failures           []Failure
	count              int
	mergeBase, headSHA string
	benchstat          string
}

func renderReport(in reportInput) string {
	var sb strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&sb, format, args...) }

	w("%s\n## Benchmark report\n\n", Marker)
	if len(in.packages) == 0 {
		w("No packages with benchmarks are affected by this pull request.\n")
		return sb.String()
	}

	w("Comparing merge base %s with PR head %s, %d interleaved runs each", shortSHA(in.mergeBase), shortSHA(in.headSHA), in.count)
	if cpu := cmp.Or(in.head.cpu, in.base.cpu); cpu != "" {
		w(" on %s", cpu)
	}
	w(". Benchmarked %s affected by this pull request.\n\n", plural(len(in.packages), "package"))
	w("Showing changes with p < %g and |Δ| ≥ %g%%. 🔴 slower or more allocations, 🟢 faster or fewer allocations.\n\n", alpha, threshold)

	for _, u := range reportUnits {
		changes, compared := compareUnit(in.base, in.head, u.unit)
		w("### %s\n\n", u.title)
		switch {
		case compared == 0:
			w("No benchmarks exist in both the base and the PR.\n\n")
			continue
		case len(changes) == 0:
			w("No significant changes across %s.\n\n", plural(compared, "benchmark"))
			continue
		}
		w("%s changed significantly out of %d compared.\n\n", plural(len(changes), "benchmark"), compared)
		w("| | Package | Benchmark | Base | PR | Δ |\n|---|---|---|--:|--:|--:|\n")
		for _, c := range changes[:min(len(changes), maxRows)] {
			icon := "🟢"
			if c.delta > 0 {
				icon = "🔴"
			}
			w("| %s | %s | %s | %s | %s | %s |\n", icon, code(displayPkg(c.key.pkg)), code(c.key.name),
				formatValue(c.base, u.unit), formatValue(c.head, u.unit), formatDelta(c.delta))
		}
		if len(changes) > maxRows {
			w("\n_%d more not shown; see the full benchstat output below._\n", len(changes)-maxRows)
		}
		w("\n")
	}

	renderOnlyIn(w, "Only in base (removed or renamed)", onlyIn(in.base, in.head))
	renderOnlyIn(w, "Only in PR (added or renamed)", onlyIn(in.head, in.base))

	if len(in.failures) > 0 {
		w("### ⚠️ Failures\n\nSome benchmarks of these packages may be missing above.\n\n")
		for _, f := range in.failures {
			w("<details><summary>%s failed to %s on %s</summary>\n\n```\n%s\n```\n</details>\n\n",
				code(displayPkg(f.ImportPath)), f.Stage, f.Checkout, strings.ReplaceAll(f.Message, "```", "'''"))
		}
	}

	report := sb.String()
	if in.benchstat == "" {
		return report
	}
	benchstat := in.benchstat
	if len(benchstat) > maxBenchstatLen {
		benchstat = benchstat[:maxBenchstatLen] + "\n… (truncated)"
	}
	details := "<details><summary>Full benchstat output</summary>\n\n```\n" +
		strings.TrimRight(strings.ReplaceAll(benchstat, "```", "'''"), "\n") + "\n```\n</details>\n"
	if len(report)+len(details) > maxReportLen {
		return report + "_The full benchstat output is too large to include here; see the workflow run artifacts._\n"
	}
	return report + details
}

func renderOnlyIn(w func(string, ...any), title string, keys []benchKey) {
	if len(keys) == 0 {
		return
	}
	w("### %s\n\n", title)
	for _, key := range keys[:min(len(keys), maxRows)] {
		w("- %s %s\n", code(displayPkg(key.pkg)), code(key.name))
	}
	if len(keys) > maxRows {
		w("- …and %d more\n", len(keys)-maxRows)
	}
	w("\n")
}

func formatValue(v float64, unit string) string {
	return benchunit.Scale(v, benchunit.ClassOf(unit))
}

func formatDelta(delta float64) string {
	if math.IsInf(delta, 1) {
		return "+∞"
	}
	return fmt.Sprintf("%+.2f%%", delta)
}

func displayPkg(pkg string) string {
	return strings.TrimPrefix(pkg, "github.com/grafana/alloy/")
}

// code formats s as inline code in a Markdown table cell.
func code(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	if strings.Contains(s, "`") {
		return s
	}
	return "`" + s + "`"
}

func shortSHA(sha string) string {
	return "`" + sha[:min(len(sha), 10)] + "`"
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
