package benchdiff

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchmath"
	"golang.org/x/perf/benchunit"
)

// Marker identifies the benchmark report comment on a pull request.
const Marker = "<!-- alloy-benchmark-report -->"

// reportUnits are the units included in the report, in order.
var reportUnits = []struct {
	unit, title string
}{
	{"sec/op", "CPU time (sec/op)"},
	{"allocs/op", "Allocations (allocs/op)"},
}

const (
	maxBenchstatLen = 30_000
	maxReportLen    = 60_000
)

type reportFlags struct {
	baseResults string
	headResults string
	meta        string
	packages    string
	benchstat   string
	mergeBase   string
	headSHA     string
	runner      string
	threshold   float64
	alpha       float64
	maxRows     int
	out         string
}

func reportCommand() *cobra.Command {
	var f reportFlags

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render a Markdown report of significant benchmark changes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReport(f)
		},
	}

	cmd.Flags().StringVar(&f.baseResults, "base-results", "", "Benchmark results of the merge base")
	cmd.Flags().StringVar(&f.headResults, "head-results", "", "Benchmark results of the head")
	cmd.Flags().StringVar(&f.meta, "meta", "", "meta.json written by the run subcommand")
	cmd.Flags().StringVar(&f.packages, "packages", "", "Packages JSON file written by the packages subcommand")
	cmd.Flags().StringVar(&f.benchstat, "benchstat", "", "Full benchstat output to include in the report")
	cmd.Flags().StringVar(&f.mergeBase, "merge-base", "", "Merge base commit")
	cmd.Flags().StringVar(&f.headSHA, "head-sha", "", "Head commit")
	cmd.Flags().StringVar(&f.runner, "runner", "", "Name of the runner the benchmarks ran on")
	cmd.Flags().Float64Var(&f.threshold, "threshold", 5, "Minimum change, in percent, to report")
	cmd.Flags().Float64Var(&f.alpha, "alpha", 0.05, "Significance level")
	cmd.Flags().IntVar(&f.maxRows, "max-rows", 30, "Maximum number of rows per table")
	cmd.Flags().StringVar(&f.out, "out", "", "Output Markdown file (default: stdout)")
	for _, name := range []string{"base-results", "head-results", "packages"} {
		_ = cmd.MarkFlagRequired(name)
	}

	return cmd
}

func runReport(f reportFlags) error {
	var in reportInput
	var err error

	if in.base, err = readResults(f.baseResults); err != nil {
		return err
	}
	if in.head, err = readResults(f.headResults); err != nil {
		return err
	}
	if err := readJSON(f.packages, &in.selection); err != nil {
		return fmt.Errorf("reading packages: %w", err)
	}
	if f.meta != "" {
		if err := readJSON(f.meta, &in.meta); err != nil {
			return fmt.Errorf("reading meta: %w", err)
		}
	}
	if f.benchstat != "" {
		data, err := os.ReadFile(f.benchstat)
		if err != nil {
			return err
		}
		in.benchstat = string(data)
	}
	in.mergeBase, in.headSHA, in.runner = f.mergeBase, f.headSHA, f.runner
	in.threshold, in.alpha, in.maxRows = f.threshold, f.alpha, f.maxRows

	report := renderReport(in)
	if f.out == "" {
		_, err = os.Stdout.WriteString(report)
		return err
	}
	return os.WriteFile(f.out, []byte(report), 0o644)
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
		units := res.samples[key]
		if units == nil {
			units = map[string][]float64{}
			res.samples[key] = units
		}
		for _, v := range rec.Values {
			units[v.Unit] = append(units[v.Unit], v.Value)
		}
	}
	return res, r.Err()
}

type change struct {
	key        benchKey
	base, head float64
	// delta is the relative change from base to head, in percent.
	delta float64
	p     float64
}

// compareUnit returns the significant changes for unit, regressions before
// improvements and each ordered by the size of the change, and the number of
// benchmarks compared.
func compareUnit(base, head results, unit string, alpha, threshold float64) ([]change, int) {
	var changes []change
	compared := 0
	for key, baseUnits := range base.samples {
		headUnits, ok := head.samples[key]
		if !ok || len(baseUnits[unit]) == 0 || len(headUnits[unit]) == 0 {
			continue
		}
		compared++

		bs := benchmath.NewSample(baseUnits[unit], &benchmath.DefaultThresholds)
		hs := benchmath.NewSample(headUnits[unit], &benchmath.DefaultThresholds)
		cmp := benchmath.AssumeNothing.Compare(bs, hs)
		if cmp.P >= alpha {
			continue
		}

		c := change{
			key:  key,
			base: benchmath.AssumeNothing.Summary(bs, 0.95).Center,
			head: benchmath.AssumeNothing.Summary(hs, 0.95).Center,
			p:    cmp.P,
		}
		switch c.base {
		case c.head:
			continue
		case 0:
			c.delta = math.Inf(1)
		default:
			c.delta = (c.head - c.base) / c.base * 100
		}
		if math.Abs(c.delta) < threshold {
			continue
		}
		changes = append(changes, c)
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
	base, head results
	selection  Selection
	meta       Meta
	benchstat  string

	mergeBase, headSHA, runner string
	threshold, alpha           float64
	maxRows                    int
}

func renderReport(in reportInput) string {
	var sb strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&sb, format, args...) }

	w("%s\n## Benchmark report\n\n", Marker)

	if len(in.selection.Packages) == 0 {
		w("No packages with benchmarks are affected by this pull request.\n")
		return sb.String()
	}

	w("Comparing merge base %s with PR head %s", shortSHA(in.mergeBase), shortSHA(in.headSHA))
	if in.meta.Count > 0 {
		w(", %d interleaved runs each", in.meta.Count)
	}
	if in.runner != "" {
		w(" on `%s`", in.runner)
	}
	if cpu := firstNonEmpty(in.head.cpu, in.base.cpu); cpu != "" {
		w(" (%s)", cpu)
	}
	w(".\n\n")

	reasons := map[string]int{}
	for _, p := range in.selection.Packages {
		reasons[p.Reason]++
	}
	w("Benchmarked %s", plural(len(in.selection.Packages), "package"))
	var parts []string
	for _, r := range []struct{ reason, label string }{
		{ReasonDirect, "changed directly"},
		{ReasonReverseDep, "importing changed packages"},
		{ReasonGoMod, "in a module whose go.mod changed"},
	} {
		if n := reasons[r.reason]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, r.label))
		}
	}
	if len(parts) > 0 {
		w(" (%s)", strings.Join(parts, ", "))
	}
	w(". Showing changes with p < %g and |Δ| ≥ %g%%. 🔴 slower or more allocations, 🟢 faster or fewer allocations.\n\n", in.alpha, in.threshold)

	for _, u := range reportUnits {
		changes, compared := compareUnit(in.base, in.head, u.unit, in.alpha, in.threshold)
		w("### %s\n\n", u.title)
		if len(changes) == 0 {
			w("No significant changes across %s.\n\n", plural(compared, "benchmark"))
			continue
		}
		w("%s changed significantly out of %d compared.\n\n", plural(len(changes), "benchmark"), compared)
		w("| | Package | Benchmark | Base | PR | Δ |\n|---|---|---|--:|--:|--:|\n")
		rows := limitRows(changes, in.maxRows)
		for _, c := range rows {
			icon := "🟢"
			if c.delta > 0 {
				icon = "🔴"
			}
			w("| %s | %s | %s | %s | %s | %s |\n", icon, code(displayPkg(c.key.pkg)), code(c.key.name),
				formatValue(c.base, u.unit), formatValue(c.head, u.unit), formatDelta(c.delta))
		}
		if hidden := len(changes) - len(rows); hidden > 0 {
			w("\n_%d more not shown; see the full benchstat output below._\n", hidden)
		}
		w("\n")
	}

	renderOnlyIn(w, "Only in base (removed or renamed)", onlyIn(in.base, in.head), in.maxRows)
	renderOnlyIn(w, "Only in PR (added or renamed)", onlyIn(in.head, in.base), in.maxRows)

	if len(in.meta.Failures) > 0 {
		w("### ⚠️ Failures\n\n")
		w("These packages couldn't be fully benchmarked, so some of their benchmarks may be missing above.\n\n")
		for _, fl := range in.meta.Failures {
			w("<details><summary>%s failed to %s on %s</summary>\n\n```\n%s\n```\n</details>\n\n",
				code(displayPkg(fl.ImportPath)), fl.Stage, fl.Tree, strings.ReplaceAll(fl.Message, "```", "'''"))
		}
	}

	report := sb.String()
	if in.benchstat != "" {
		details := renderBenchstat(in.benchstat)
		if len(report)+len(details) <= maxReportLen {
			report += details
		} else {
			report += "_The full benchstat output is too large for this comment; download it from the workflow run artifacts._\n"
		}
	}
	return report
}

func renderOnlyIn(w func(string, ...any), title string, keys []benchKey, maxRows int) {
	if len(keys) == 0 {
		return
	}
	w("### %s\n\n", title)
	for _, key := range limitRows(keys, maxRows) {
		w("- %s %s\n", code(displayPkg(key.pkg)), code(key.name))
	}
	if hidden := len(keys) - min(len(keys), maxRows); hidden > 0 {
		w("- …and %d more\n", hidden)
	}
	w("\n")
}

func renderBenchstat(out string) string {
	if len(out) > maxBenchstatLen {
		out = out[:maxBenchstatLen] + "\n… (truncated, see the workflow run artifacts)"
	}
	out = strings.ReplaceAll(out, "```", "'''")
	return "<details><summary>Full benchstat output</summary>\n\n```\n" + strings.TrimRight(out, "\n") + "\n```\n</details>\n"
}

func limitRows[T any](rows []T, n int) []T {
	if n > 0 && len(rows) > n {
		return rows[:n]
	}
	return rows
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

// code formats s as inline code inside a Markdown table cell.
func code(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	if strings.Contains(s, "`") {
		return s
	}
	return "`" + s + "`"
}

func shortSHA(sha string) string {
	if sha == "" {
		return "(unknown)"
	}
	if len(sha) > 10 {
		sha = sha[:10]
	}
	return "`" + sha + "`"
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
