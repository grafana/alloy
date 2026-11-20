package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	docsDir       = "docs/sources/reference/components/otelcol"
	allowlistPath = "skills/key-deps-update/scripts/docs/otel-stability/allowlist.txt"
	alloyOnlyLine = "Upstream stability: none, this component is Alloy only."
	stabilityRef  = `[otel-stability]: https://github.com/open-telemetry/opentelemetry-collector/blob/{{< param "OTEL_VERSION" >}}/docs/component-stability.md`
)

// Upstream levels from highest to lowest. deprecated and unmaintained sit outside the ladder.
var levelRank = map[string]int{"stable": 4, "beta": 3, "alpha": 2, "development": 1}

var stageForLevel = map[string]string{
	"stable":      "general-availability",
	"beta":        "public-preview",
	"alpha":       "experimental",
	"development": "experimental",
}

var (
	stageRe        = regexp.MustCompile(`(?m)^\s+stage:\s*(\S+)`)
	wrapperRe      = regexp.MustCompile("wrapper over the upstream[^\n]*?\\[(`?[^\\]`]+`?)\\]\\[\\]")
	docLineRe      = regexp.MustCompile(`Upstream \[stability\]\[otel-stability\]: ([^\n]+?)\.\s*\n`)
	levelClauseRe  = regexp.MustCompile(`\b(development|alpha|beta|stable|deprecated|unmaintained)\b(?: for )?`)
	signalSplitRe  = regexp.MustCompile(`\s*,\s*(?:and\s+)?|\s+and\s+`)
	contribURLRe   = regexp.MustCompile(`^https://github\.com/open-telemetry/opentelemetry-collector-contrib/tree/[^/]+/(.+?)/?$`)
	coreURLRe      = regexp.MustCompile(`^https://github\.com/open-telemetry/opentelemetry-collector/tree/[^/]+/(.+?)/?$`)
	contribModRoot = "github.com/open-telemetry/opentelemetry-collector-contrib/"
	coreModRoot    = "go.opentelemetry.io/collector/"
)

// stability maps an upstream level to the signals at that level, as in metadata.yaml status.stability.
type stability map[string][]string

type allowEntry struct {
	stage string
	level string
}

type report struct {
	missing, mismatch, stage, deprecated, grandfatheredChanged, stale, unresolved, unlisted, skipped []string
}

func main() {
	repo := flag.String("repo", "", "Path to the Alloy repository root (default: git top-level of the current directory)")
	flag.Parse()

	root := *repo
	if root == "" {
		out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err != nil {
			fatalf("can't find the repository root, pass --repo: %v", err)
		}
		root = strings.TrimSpace(string(out))
	}

	allow, err := readAllowlist(filepath.Join(root, allowlistPath))
	if err != nil {
		fatalf("reading allowlist: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(root, docsDir, "otelcol.*.md"))
	if err != nil || len(files) == 0 {
		fatalf("no otelcol docs found under %s", filepath.Join(root, docsDir))
	}
	sort.Strings(files)

	var r report
	seen := map[string]bool{}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		seen[name] = true
		content, err := os.ReadFile(f)
		if err != nil {
			fatalf("reading %s: %v", f, err)
		}
		checkDoc(root, name, string(content), allow, &r)
	}
	for name := range allow {
		if !seen[name] {
			r.stale = append(r.stale, fmt.Sprintf("%s: in the allowlist but has no doc. Remove the entry.", name))
		}
	}

	printReport(r)
}

func checkDoc(root, name, doc string, allow map[string]allowEntry, r *report) {
	stage := ""
	if m := stageRe.FindStringSubmatch(doc); m != nil {
		stage = m[1]
	}
	if stage == "" {
		r.skipped = append(r.skipped, fmt.Sprintf("%s: no `stage` label (community component)", name))
		return
	}

	label := ""
	if m := wrapperRe.FindStringSubmatch(doc); m != nil {
		label = m[1]
	}
	if label == "" {
		if strings.Contains(doc, "wrapper over the upstream") {
			r.unresolved = append(r.unresolved, fmt.Sprintf("%s: the wrapper sentence has no reference-style link to the upstream component, so the upstream module can't be found. Add one.", name))
			return
		}
		if !strings.Contains(doc, alloyOnlyLine) {
			r.missing = append(r.missing, fmt.Sprintf("%s: no upstream component found. If it's Alloy only, add to its note admonition: %q", name, alloyOnlyLine))
		}
		return
	}

	url := linkTarget(doc, label)
	module := moduleFromURL(url)
	if module == "" {
		r.unresolved = append(r.unresolved, fmt.Sprintf("%s: can't map the upstream link %q to a Go module", name, url))
		return
	}
	upstream, version, err := upstreamStability(root, module)
	if err != nil {
		r.unresolved = append(r.unresolved, fmt.Sprintf("%s: %s: %v", name, module, err))
		return
	}

	m := docLineRe.FindStringSubmatch(doc)
	if m == nil || !strings.Contains(doc, "[otel-stability]:") {
		r.missing = append(r.missing, fmt.Sprintf("%s (%s@%s): add %q and the link definition %s. Drop any signal the Alloy component doesn't expose.",
			name, module, version, "Upstream [stability][otel-stability]: "+formatLevels(upstream)+".", stabilityRef))
		return
	}

	documented := parseDocLevels(m[1])
	expected := expectedFor(documented, upstream)
	if formatLevels(expected) != formatLevels(documented) {
		r.mismatch = append(r.mismatch, fmt.Sprintf("%s (%s@%s): docs say %q, upstream says %q", name, module, version, formatLevels(documented), formatLevels(expected)))
	}
	if extra := unlistedSignals(documented, upstream); len(extra) > 0 {
		r.unlisted = append(r.unlisted, fmt.Sprintf("%s: upstream also has %s. Confirm the Alloy component doesn't expose these.", name, strings.Join(extra, ", ")))
	}

	for _, lvl := range []string{"deprecated", "unmaintained"} {
		if len(expected[lvl]) > 0 {
			r.deprecated = append(r.deprecated, fmt.Sprintf("%s: upstream marks %s as %s. This is user-facing and needs a release note.", name, strings.Join(expected[lvl], ", "), lvl))
		}
	}

	top := highestLevel(expected)
	if top == "" {
		return
	}
	want := stageForLevel[top]
	entry, allowed := allow[name]
	switch {
	case want == stage && allowed:
		r.stale = append(r.stale, fmt.Sprintf("%s: stage %s now matches upstream %s. Remove the allowlist entry.", name, stage, top))
	case want == stage:
	case allowed && entry.stage == stage && entry.level == top:
	case allowed:
		r.grandfatheredChanged = append(r.grandfatheredChanged, fmt.Sprintf("%s: allowlisted as %s/%s, now %s/%s (Alloy/upstream). Upstream moved, so review the Alloy stage.", name, entry.stage, entry.level, stage, top))
	default:
		r.stage = append(r.stage, fmt.Sprintf("%s: Alloy stage is %s but upstream's highest level is %s, which maps to %s. Allowlist line, only with maintainer approval: `%s %s %s`", name, stage, top, want, name, stage, top))
	}
}

// linkTarget returns the URL for a reference-style link definition such as "[`label`]: url".
func linkTarget(doc, label string) string {
	re := regexp.MustCompile(`(?m)^\[` + regexp.QuoteMeta(label) + `\]:\s*(.+?)\s*$`)
	if m := re.FindStringSubmatch(doc); m != nil {
		return m[1]
	}
	return ""
}

func moduleFromURL(url string) string {
	if m := contribURLRe.FindStringSubmatch(url); m != nil {
		return contribModRoot + m[1]
	}
	if m := coreURLRe.FindStringSubmatch(url); m != nil {
		return coreModRoot + m[1]
	}
	return ""
}

// upstreamStability reads status.stability from the module's metadata.yaml at the version Alloy's go.mod selects.
func upstreamStability(root, module string) (stability, string, error) {
	cmd := exec.Command("go", "mod", "download", "-json", module)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("go mod download: %v", err)
	}
	var info struct{ Dir, Version, Error string }
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, "", err
	}
	if info.Error != "" {
		return nil, "", fmt.Errorf("%s", info.Error)
	}
	raw, err := os.ReadFile(filepath.Join(info.Dir, "metadata.yaml"))
	if err != nil {
		return nil, info.Version, err
	}
	var md struct {
		Status struct {
			Stability stability `yaml:"stability"`
		} `yaml:"status"`
	}
	if err := yaml.Unmarshal(raw, &md); err != nil {
		return nil, info.Version, err
	}
	if len(md.Status.Stability) == 0 {
		return nil, info.Version, fmt.Errorf("metadata.yaml has no status.stability")
	}
	return md.Status.Stability, info.Version, nil
}

// parseDocLevels parses "beta for traces and logs, alpha for metrics" or "beta".
// A level with no signals is stored with an empty signal list.
func parseDocLevels(s string) stability {
	out := stability{}
	idx := levelClauseRe.FindAllStringSubmatchIndex(s, -1)
	for i, m := range idx {
		level := s[m[2]:m[3]]
		end := len(s)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		rest := strings.Trim(strings.TrimSpace(s[m[1]:end]), ",")
		if _, ok := out[level]; !ok {
			out[level] = []string{}
		}
		if rest == "" {
			continue
		}
		for _, sig := range signalSplitRe.Split(strings.TrimSpace(rest), -1) {
			if sig = strings.TrimSpace(sig); sig != "" {
				out[level] = append(out[level], strings.ReplaceAll(sig, " ", "_"))
			}
		}
	}
	return out
}

// expectedFor returns upstream levels restricted to the signals the docs list.
// A level with no signals (extensions) means every upstream signal.
func expectedFor(documented, upstream stability) stability {
	listed := map[string]bool{}
	bare := false
	for _, sigs := range documented {
		if len(sigs) == 0 {
			bare = true
		}
		for _, s := range sigs {
			listed[s] = true
		}
	}
	if bare && len(listed) == 0 {
		out := stability{}
		for lvl := range upstream {
			out[lvl] = []string{}
		}
		return out
	}
	out := stability{}
	for lvl, sigs := range upstream {
		for _, s := range sigs {
			if listed[s] {
				out[lvl] = append(out[lvl], s)
			}
		}
	}
	for s := range listed {
		if levelOf(upstream, s) == "" {
			out["(not upstream)"] = append(out["(not upstream)"], s)
		}
	}
	return out
}

func unlistedSignals(documented, upstream stability) []string {
	listed := map[string]bool{}
	bare := false
	for _, sigs := range documented {
		if len(sigs) == 0 {
			bare = true
		}
		for _, s := range sigs {
			listed[s] = true
		}
	}
	if bare && len(listed) == 0 {
		return nil
	}
	var extra []string
	for _, lvl := range sortedLevels(upstream) {
		for _, s := range upstream[lvl] {
			if !listed[s] {
				extra = append(extra, fmt.Sprintf("%s (%s)", strings.ReplaceAll(s, "_", " "), lvl))
			}
		}
	}
	return extra
}

func levelOf(st stability, signal string) string {
	for lvl, sigs := range st {
		if slices.Contains(sigs, signal) {
			return lvl
		}
	}
	return ""
}

func highestLevel(st stability) string {
	best, rank := "", 0
	for lvl := range st {
		if levelRank[lvl] > rank {
			best, rank = lvl, levelRank[lvl]
		}
	}
	return best
}

// sortedLevels orders levels from highest to lowest, with off-ladder levels last.
func sortedLevels(st stability) []string {
	lvls := make([]string, 0, len(st))
	for lvl := range st {
		lvls = append(lvls, lvl)
	}
	sort.Slice(lvls, func(i, j int) bool {
		if levelRank[lvls[i]] != levelRank[lvls[j]] {
			return levelRank[lvls[i]] > levelRank[lvls[j]]
		}
		return lvls[i] < lvls[j]
	})
	return lvls
}

// formatLevels renders levels the way the docs write them, e.g. "beta for traces and logs, alpha for metrics".
// Extension-only stability renders as the bare level.
func formatLevels(st stability) string {
	var parts []string
	for _, lvl := range sortedLevels(st) {
		sigs := st[lvl]
		if len(sigs) == 0 || (len(sigs) == 1 && sigs[0] == "extension") {
			parts = append(parts, lvl)
			continue
		}
		sorted := slices.Clone(sigs)
		sort.SliceStable(sorted, func(i, j int) bool { return signalRank(sorted[i]) < signalRank(sorted[j]) })
		display := make([]string, len(sorted))
		for i, s := range sorted {
			display[i] = strings.ReplaceAll(s, "_", " ")
		}
		parts = append(parts, lvl+" for "+joinList(display))
	}
	return strings.Join(parts, ", ")
}

// signalRank orders signals as traces, metrics, logs, profiles, then connector pairs alphabetically.
func signalRank(s string) string {
	switch s {
	case "traces":
		return "0"
	case "metrics":
		return "1"
	case "logs":
		return "2"
	case "profiles":
		return "3"
	}
	return "4" + s
}

func joinList(items []string) string {
	switch len(items) {
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

// readAllowlist reads "<component> <alloy-stage> <upstream-level>" lines. Blank lines and # comments are ignored.
func readAllowlist(path string) (map[string]allowEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]allowEntry{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("line %d: want 3 fields, got %q", n, line)
		}
		out[fields[0]] = allowEntry{stage: fields[1], level: fields[2]}
	}
	return out, sc.Err()
}

func printReport(r report) {
	sections := []struct {
		title string
		items []string
	}{
		{"Docs disagree with upstream (fix the docs line)", r.mismatch},
		{"Upstream deprecated or unmaintained (release note needed)", r.deprecated},
		{"Alloy stage doesn't match upstream (needs a decision)", r.stage},
		{"Grandfathered component whose upstream level moved (needs a decision)", r.grandfatheredChanged},
		{"Missing upstream stability line", r.missing},
		{"Can't resolve upstream component", r.unresolved},
		{"Stale allowlist entries", r.stale},
		{"Upstream signals not listed in the docs (confirm)", r.unlisted},
		{"Skipped", r.skipped},
	}
	total := 0
	for _, s := range sections {
		if len(s.items) == 0 {
			continue
		}
		fmt.Printf("## %s (%d)\n\n", s.title, len(s.items))
		for _, it := range s.items {
			fmt.Printf("- %s\n", it)
		}
		fmt.Println()
		if s.title != "Skipped" {
			total += len(s.items)
		}
	}
	if total == 0 {
		fmt.Println("All otelcol docs match upstream stability.")
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
