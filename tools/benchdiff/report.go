package benchdiff

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"slices"
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

var reportUnits = []struct{ unit, metric string }{
	{"sec/op", "CPU (sec/op)"},
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
	metric, unit string
	base, head   float64
	delta        float64 // percent
}

// benchChanges is a benchmark with its significant changes, one per unit.
type benchChanges struct {
	key     benchKey
	changes []change
}

func (b benchChanges) regressed() bool {
	return slices.ContainsFunc(b.changes, func(c change) bool { return c.delta > 0 })
}

func (b benchChanges) maxDelta() float64 {
	var m float64
	for _, c := range b.changes {
		m = max(m, math.Abs(c.delta))
	}
	return m
}

// significantChanges returns the benchmarks with significant changes, those
// with regressions first and each group ordered by the largest change, and the
// number of benchmarks compared.
func significantChanges(base, head results) ([]benchChanges, int) {
	var benches []benchChanges
	compared := 0
	for key, baseUnits := range base.samples {
		headUnits, ok := head.samples[key]
		if !ok {
			continue
		}
		compared++

		b := benchChanges{key: key}
		for _, u := range reportUnits {
			if c, ok := compareSamples(baseUnits[u.unit], headUnits[u.unit]); ok {
				c.metric, c.unit = u.metric, u.unit
				b.changes = append(b.changes, c)
			}
		}
		if len(b.changes) > 0 {
			benches = append(benches, b)
		}
	}

	sort.Slice(benches, func(i, j int) bool {
		a, b := benches[i], benches[j]
		if a.regressed() != b.regressed() {
			return a.regressed()
		}
		if a.maxDelta() != b.maxDelta() {
			return a.maxDelta() > b.maxDelta()
		}
		return lessKey(a.key, b.key)
	})
	return benches, compared
}

// compareSamples reports whether head differs significantly from base.
func compareSamples(baseValues, headValues []float64) (change, bool) {
	if len(baseValues) == 0 || len(headValues) == 0 {
		return change{}, false
	}
	bs := benchmath.NewSample(baseValues, &benchmath.DefaultThresholds)
	hs := benchmath.NewSample(headValues, &benchmath.DefaultThresholds)
	if benchmath.AssumeNothing.Compare(bs, hs).P >= alpha {
		return change{}, false
	}

	c := change{
		base: benchmath.AssumeNothing.Summary(bs, 0.95).Center,
		head: benchmath.AssumeNothing.Summary(hs, 0.95).Center,
	}
	switch c.base {
	case c.head:
		return change{}, false
	case 0:
		c.delta = math.Inf(1)
	default:
		c.delta = (c.head - c.base) / c.base * 100
	}
	return c, math.Abs(c.delta) >= threshold
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

	benches, compared := significantChanges(in.base, in.head)
	w("### Significant changes\n\n")
	switch {
	case compared == 0:
		w("No benchmarks exist in both the base and the PR.\n\n")
	case len(benches) == 0:
		w("No significant changes in CPU or allocations across %s.\n\n", plural(compared, "benchmark"))
	default:
		w("%s changed significantly out of %d compared.\n\n", plural(len(benches), "benchmark"), compared)
		w("| | Package | Benchmark | Metric | Base | PR | Δ |\n|---|---|---|---|--:|--:|--:|\n")
		for _, b := range benches[:min(len(benches), maxRows)] {
			for _, c := range b.changes {
				icon := "🟢"
				if c.delta > 0 {
					icon = "🔴"
				}
				w("| %s | %s | %s | %s | %s | %s | %s |\n", icon, code(displayPkg(b.key.pkg)), code(b.key.name), c.metric,
					formatValue(c.base, c.unit), formatValue(c.head, c.unit), formatDelta(c.delta))
			}
		}
		if len(benches) > maxRows {
			w("\n_%d more not shown; see the full benchstat output below._\n", len(benches)-maxRows)
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
