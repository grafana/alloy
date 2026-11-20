package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const componentsDir = "internal/component/prometheus/exporter"

// exporterModules maps each prometheus.exporter.* component directory to the Go module that
// provides the exporter it embeds.
var exporterModules = map[string]string{
	"apache":        "github.com/Lusitaniae/apache_exporter",
	"azure":         "github.com/webdevops/azure-metrics-exporter",
	"blackbox":      "github.com/prometheus/blackbox_exporter",
	"cadvisor":      "github.com/google/cadvisor",
	"cloudwatch":    "github.com/prometheus-community/yet-another-cloudwatch-exporter",
	"consul":        "github.com/prometheus/consul_exporter",
	"databricks":    "github.com/grafana/databricks-prometheus-exporter",
	"dnsmasq":       "github.com/google/dnsmasq_exporter",
	"elasticsearch": "github.com/prometheus-community/elasticsearch_exporter",
	"gcp":           "github.com/prometheus-community/stackdriver_exporter",
	"github":        "github.com/githubexporter/github-exporter",
	"kafka":         "github.com/grafana/kafka_exporter",
	"memcached":     "github.com/prometheus/memcached_exporter",
	"mongodb":       "github.com/percona/mongodb_exporter",
	"mssql":         "github.com/burningalchemist/sql_exporter",
	"mysql":         "github.com/prometheus/mysqld_exporter",
	"oracledb":      "github.com/oracle/oracle-db-appdev-monitoring",
	"postgres":      "github.com/prometheus-community/postgres_exporter",
	"process":       "github.com/ncabatoff/process-exporter",
	"redis":         "github.com/oliver006/redis_exporter",
	"snmp":          "github.com/prometheus/snmp_exporter",
	"snowflake":     "github.com/grafana/snowflake-prometheus-exporter",
	"squid":         "github.com/boynux/squid-exporter",
	"statsd":        "github.com/prometheus/statsd_exporter",
	"unix":          "github.com/prometheus/node_exporter",
	"windows":       "github.com/prometheus-community/windows_exporter",
}

// notExporters are directories under componentsDir that don't wrap a third-party exporter.
var notExporters = map[string]bool{
	"common": true,
	"self":   true,
	"static": true,
	"tests":  true,
}

var (
	pseudoVersionRe = regexp.MustCompile(`[-.](\d{14})-([0-9a-f]{12})$`)
	staticImportRe  = regexp.MustCompile(`"github\.com/grafana/alloy/(internal/static/integrations/[a-z0-9_]+)"`)
	githubRepoRe    = regexp.MustCompile(`^github\.com/([^/]+/[^/]+)`)
	semverRe        = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.-]+)?$`)
)

type modFile struct {
	Require []struct {
		Path    string
		Version string
	}
	Replace []struct {
		Old struct{ Path string }
		New struct {
			Path    string
			Version string
		}
	}
}

// pin is how go.mod resolves one exporter module.
type pin struct {
	Module         string
	Required       string
	ReplacePath    string
	ReplaceVersion string
	ReplaceComment string
}

// Effective returns the module path and version Alloy actually builds.
func (p pin) Effective() (string, string) {
	if p.ReplacePath != "" {
		return p.ReplacePath, p.ReplaceVersion
	}
	return p.Module, p.Required
}

type repoInfo struct {
	FullName      string `json:"full_name"`
	Archived      bool   `json:"archived"`
	Fork          bool   `json:"fork"`
	DefaultBranch string `json:"default_branch"`
	PushedAt      string `json:"pushed_at"`
	Parent        *struct {
		FullName string `json:"full_name"`
	} `json:"parent"`
}

type release struct {
	Tag       string
	Published string
}

// drift compares the pinned commit with upstream's default branch.
type drift struct {
	MergeBase     string
	MergeBaseDate string
	ForkOnly      int
	UpstreamOnly  int
}

type exporter struct {
	Component    string
	Paths        []string
	Pin          pin
	PinCommit    string
	PinDate      string
	Upstream     string
	ForkRepo     string
	UpstreamInfo *repoInfo
	ForkInfo     *repoInfo
	Latest       *release
	Drift        *drift
	BranchHeads  []string
	Audit        bool
	Reason       string
	Errors       []string
}

func (e *exporter) IsFork() bool { return e.ForkRepo != "" }

func main() {
	repo := flag.String("repo", ".", "Path to the root of an Alloy checkout")
	only := flag.String("exporter", "", "Comma-separated component names to include (default: all)")
	flag.Parse()

	if err := run(*repo, *only, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(repo, only string, out io.Writer) error {
	components, problems, err := discoverComponents(repo)
	if err != nil {
		return err
	}
	if only != "" {
		components = filterComponents(components, strings.Split(only, ","))
		if len(components) == 0 {
			return fmt.Errorf("no exporter components match %q", only)
		}
	}

	pins, err := readPins(repo)
	if err != nil {
		return err
	}

	exporters := make([]*exporter, 0, len(components))
	for _, c := range components {
		e := &exporter{Component: c}
		e.Paths = append([]string{filepath.Join(componentsDir, c)}, staticIntegrations(repo, c)...)
		mod := exporterModules[c]
		p, ok := pins[mod]
		if !ok {
			problems = append(problems, fmt.Sprintf("`%s`: module `%s` isn't required in go.mod", c, mod))
			continue
		}
		if !importsModule(repo, e.Paths, mod) {
			problems = append(problems, fmt.Sprintf("`%s`: none of %s import `%s`; update exporterModules", c, strings.Join(e.Paths, ", "), mod))
		}
		e.Pin = p
		exporters = append(exporters, e)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, e := range exporters {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			inspect(e)
		})
	}
	wg.Wait()

	return render(out, exporters, problems)
}

// discoverComponents lists exporter component directories and reports any without a module mapping.
func discoverComponents(repo string) ([]string, []string, error) {
	entries, err := os.ReadDir(filepath.Join(repo, componentsDir))
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s (is --repo the root of an Alloy checkout?): %w", componentsDir, err)
	}
	var components, problems []string
	for _, entry := range entries {
		if !entry.IsDir() || notExporters[entry.Name()] {
			continue
		}
		if _, ok := exporterModules[entry.Name()]; !ok {
			problems = append(problems, fmt.Sprintf("`%s`: no entry in exporterModules; add one or list it in notExporters", entry.Name()))
			continue
		}
		components = append(components, entry.Name())
	}
	sort.Strings(components)
	return components, problems, nil
}

func filterComponents(components, names []string) []string {
	want := map[string]bool{}
	for _, n := range names {
		want[strings.TrimSpace(n)] = true
	}
	var out []string
	for _, c := range components {
		if want[c] {
			out = append(out, c)
		}
	}
	return out
}

// readPins resolves every required module in go.mod, including replace targets and the
// comment above each replace.
func readPins(repo string) (map[string]pin, error) {
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = repo
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go mod edit -json: %w", err)
	}
	var mf modFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		return nil, fmt.Errorf("parsing go mod edit output: %w", err)
	}

	f, err := os.Open(filepath.Join(repo, "go.mod"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	comments, err := replaceComments(f)
	if err != nil {
		return nil, err
	}

	pins := map[string]pin{}
	for _, r := range mf.Require {
		pins[r.Path] = pin{Module: r.Path, Required: r.Version}
	}
	for _, r := range mf.Replace {
		p, ok := pins[r.Old.Path]
		if !ok {
			continue
		}
		p.ReplacePath = r.New.Path
		p.ReplaceVersion = r.New.Version
		p.ReplaceComment = comments[r.Old.Path]
		pins[r.Old.Path] = p
	}
	return pins, nil
}

// replaceComments returns the comment lines directly above each single-line replace directive.
func replaceComments(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	var pending []string
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		switch {
		case strings.HasPrefix(line, "//"):
			pending = append(pending, strings.TrimSpace(strings.TrimPrefix(line, "//")))
		case strings.HasPrefix(line, "replace ") && strings.Contains(line, "=>"):
			old := strings.Fields(strings.TrimPrefix(line, "replace "))[0]
			if len(pending) > 0 {
				out[old] = strings.Join(pending, " ")
			}
			pending = nil
		default:
			pending = nil
		}
	}
	return out, s.Err()
}

// staticIntegrations returns the static integration packages a component imports.
func staticIntegrations(repo, component string) []string {
	seen := map[string]bool{}
	walkGo(filepath.Join(repo, componentsDir, component), func(src string) {
		for _, m := range staticImportRe.FindAllStringSubmatch(src, -1) {
			if !strings.HasSuffix(m[1], "/config") {
				seen[m[1]] = true
			}
		}
	})
	var out []string
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func importsModule(repo string, paths []string, module string) bool {
	found := false
	for _, p := range paths {
		walkGo(filepath.Join(repo, p), func(src string) {
			if strings.Contains(src, `"`+module+`"`) || strings.Contains(src, `"`+module+`/`) {
				found = true
			}
		})
	}
	return found
}

// walkGo calls fn with the contents of every non-test .go file directly in dir.
func walkGo(dir string, fn func(string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			fn(string(b))
		}
	}
}

// inspect fills in GitHub state and decides whether e needs an audit.
func inspect(e *exporter) {
	path, version := e.Pin.Effective()
	if m := pseudoVersionRe.FindStringSubmatch(version); m != nil {
		e.PinCommit = m[2]
		if t, err := time.Parse("20060102150405", m[1]); err == nil {
			e.PinDate = t.Format("2006-01-02")
		}
	}

	effRepo := githubRepo(path)
	if effRepo == "" {
		e.Errors = append(e.Errors, "not hosted on GitHub; inspect it by hand")
		e.Audit, e.Reason = true, "not on GitHub"
		return
	}
	effInfo, err := getRepo(effRepo)
	if err != nil {
		e.Errors = append(e.Errors, err.Error())
	}

	switch {
	case e.Pin.ReplacePath != "" && githubRepo(e.Pin.Module) != effRepo:
		// Replaced: upstream is the module we require, not the fork's GitHub parent, which can be stale.
		e.ForkRepo, e.ForkInfo = effRepo, effInfo
		e.Upstream = githubRepo(e.Pin.Module)
		if e.UpstreamInfo, err = getRepo(e.Upstream); err != nil {
			e.Errors = append(e.Errors, err.Error())
		}
	case effInfo != nil && effInfo.Fork && effInfo.Parent != nil:
		// Imported directly from a fork.
		e.ForkRepo, e.ForkInfo = effRepo, effInfo
		e.Upstream = effInfo.Parent.FullName
		if e.UpstreamInfo, err = getRepo(e.Upstream); err != nil {
			e.Errors = append(e.Errors, err.Error())
		}
	default:
		e.Upstream, e.UpstreamInfo = effRepo, effInfo
	}

	if e.Latest, err = latestRelease(e.Upstream); err != nil {
		e.Errors = append(e.Errors, err.Error())
	}

	ref := e.PinCommit
	if ref == "" {
		ref = version
	}
	if e.UpstreamInfo != nil {
		head := ref
		if e.IsFork() {
			head = strings.Split(e.ForkRepo, "/")[0] + ":" + ref
		}
		if e.Drift, err = compare(e.Upstream, e.UpstreamInfo.DefaultBranch, head); err != nil {
			e.Errors = append(e.Errors, err.Error())
		}
	}
	if e.IsFork() && e.PinCommit != "" {
		if e.BranchHeads, err = branchesWhereHead(e.ForkRepo, e.PinCommit); err != nil {
			e.Errors = append(e.Errors, err.Error())
		}
	}

	e.Audit, e.Reason = gate(e)
}

// gate decides whether an exporter is worth a per-exporter audit.
func gate(e *exporter) (bool, string) {
	_, version := e.Pin.Effective()
	switch {
	case e.IsFork():
		return true, "fork"
	case e.Latest != nil && e.PinCommit == "" && semverLess(version, e.Latest.Tag):
		return true, "behind latest release"
	case e.Latest != nil && e.PinCommit != "" && e.PinDate != "" && e.PinDate < e.Latest.Published:
		return true, "pinned to a commit older than the latest release"
	case e.Drift != nil && e.Drift.UpstreamOnly > 0 && (e.PinCommit != "" || e.Latest == nil):
		return true, "pin is behind the default branch"
	case len(e.Errors) > 0:
		return true, "lookup failed"
	default:
		return false, "current"
	}
}

func githubRepo(modulePath string) string {
	if m := githubRepoRe.FindStringSubmatch(modulePath); m != nil {
		return m[1]
	}
	return ""
}

func gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", append([]string{"api"}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func getRepo(repo string) (*repoInfo, error) {
	raw, err := gh("repos/" + repo)
	if err != nil {
		return nil, err
	}
	var info repoInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// latestRelease returns the latest GitHub release, or nil when the repo publishes none.
func latestRelease(repo string) (*release, error) {
	raw, err := gh("repos/"+repo+"/releases/latest", "--jq", `[.tag_name, .published_at[:10]] | join(" ")`)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 {
		return nil, fmt.Errorf("unexpected release output for %s: %q", repo, raw)
	}
	return &release{Tag: fields[0], Published: fields[1]}, nil
}

// compare diffs head against base on upstream. Counts include merge commits.
func compare(upstream, base, head string) (*drift, error) {
	raw, err := gh("repos/"+upstream+"/compare/"+base+"..."+head, "--jq",
		`[.merge_base_commit.sha[:7], .merge_base_commit.commit.committer.date[:10], .ahead_by, .behind_by] | map(tostring) | join(" ")`)
	if err != nil {
		return nil, err
	}
	f := strings.Fields(string(raw))
	if len(f) != 4 {
		return nil, fmt.Errorf("unexpected compare output for %s: %q", upstream, raw)
	}
	ahead, _ := strconv.Atoi(f[2])
	behind, _ := strconv.Atoi(f[3])
	return &drift{MergeBase: f[0], MergeBaseDate: f[1], ForkOnly: ahead, UpstreamOnly: behind}, nil
}

func branchesWhereHead(repo, sha string) ([]string, error) {
	raw, err := gh("repos/"+repo+"/commits/"+sha+"/branches-where-head", "--jq", ".[].name")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(raw)), nil
}

// semverLess reports whether a < b. Unparseable versions compare as not less.
func semverLess(a, b string) bool {
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	if !oka || !okb {
		return false
	}
	for i := range 3 {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseSemver(v string) ([3]int, bool) {
	m := semverRe.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}, false
	}
	var out [3]int
	for i := range 3 {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// printer writes to w and keeps the first write error.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, args ...any) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format, args...)
	}
}

func (p *printer) println(args ...any) {
	if p.err == nil {
		_, p.err = fmt.Fprintln(p.w, args...)
	}
}

func render(w io.Writer, exporters []*exporter, problems []string) error {
	o := &printer{w: w}
	o.println("# Exporter inventory")
	o.println()
	o.printf("Generated %s.\n\n", time.Now().Format("2006-01-02"))

	if len(problems) > 0 {
		o.println("## Mapping problems")
		o.println()
		o.println("Fix these in `exporterModules` before trusting the rest of the output.")
		o.println()
		for _, p := range problems {
			o.printf("- %s\n", p)
		}
		o.println()
	}

	o.println("## Summary")
	o.println()
	o.println("| Component | Builds from | Fork of | Latest upstream release | Upstream commits not in pin | Audit |")
	o.println("|---|---|---|---|---|---|")
	for _, e := range exporters {
		path, version := e.Pin.Effective()
		forkOf := "-"
		if e.IsFork() {
			forkOf = e.Upstream
		}
		latest := "none"
		if e.Latest != nil {
			latest = e.Latest.Tag + " (" + e.Latest.Published + ")"
		}
		behind := "?"
		if e.Drift != nil {
			behind = strconv.Itoa(e.Drift.UpstreamOnly)
		}
		audit := "no"
		if e.Audit {
			audit = "**yes**: " + e.Reason
		}
		o.printf("| %s | `%s %s` | %s | %s | %s | %s |\n", e.Component, path, version, forkOf, latest, behind, audit)
	}
	o.println()

	o.println("## Briefs")
	o.println()
	o.println("One per exporter that needs an audit. Hand each one to its subagent as-is.")
	for _, e := range exporters {
		if !e.Audit {
			continue
		}
		o.println()
		renderBrief(o, e)
	}

	var skipped []string
	for _, e := range exporters {
		if !e.Audit {
			skipped = append(skipped, e.Component)
		}
	}
	if len(skipped) > 0 {
		o.println()
		o.println("## Current, no audit needed")
		o.println()
		o.println(strings.Join(skipped, ", "))
	}
	return o.err
}

func renderBrief(o *printer, e *exporter) {
	path, version := e.Pin.Effective()
	o.printf("### %s\n\n", e.Component)
	o.printf("- Audit reason: %s\n", e.Reason)
	o.printf("- Alloy code: %s\n", strings.Join(wrapCode(e.Paths), ", "))
	o.printf("- go.mod require: `%s %s`\n", e.Pin.Module, e.Pin.Required)
	if e.Pin.ReplacePath != "" {
		o.printf("- go.mod replace: `%s %s`. The require version is cosmetic; the fork's base is what matters.\n", path, version)
		if e.Pin.ReplaceComment != "" {
			o.printf("- Replace comment: %s\n", e.Pin.ReplaceComment)
		}
	}
	if e.PinCommit != "" {
		o.printf("- Pinned commit: `%s` (%s)\n", e.PinCommit, e.PinDate)
	}
	o.printf("- Upstream: %s%s\n", e.Upstream, repoNotes(e.UpstreamInfo))
	if e.IsFork() {
		o.printf("- Fork: %s%s\n", e.ForkRepo, repoNotes(e.ForkInfo))
		if e.ForkInfo != nil && e.ForkInfo.Parent != nil && e.ForkInfo.Parent.FullName != e.Upstream {
			o.printf("- GitHub lists the fork's parent as %s, not the upstream above.\n", e.ForkInfo.Parent.FullName)
		}
		if e.PinCommit != "" {
			if len(e.BranchHeads) > 0 {
				o.printf("- The pin is the head of fork branch(es): %s\n", strings.Join(wrapCode(e.BranchHeads), ", "))
			} else {
				o.println("- The pin isn't the head of any fork branch. Find which branch contains it and what's newer.")
			}
		}
	}
	if e.Latest != nil {
		o.printf("- Latest upstream release: %s (%s)\n", e.Latest.Tag, e.Latest.Published)
	} else {
		o.println("- Upstream publishes no GitHub releases.")
	}
	if d := e.Drift; d != nil && e.UpstreamInfo != nil {
		o.printf("- Against upstream `%s`: merge base `%s` (%s), %d commits only in the pin, %d only upstream (counts include merges)\n",
			e.UpstreamInfo.DefaultBranch, d.MergeBase, d.MergeBaseDate, d.ForkOnly, d.UpstreamOnly)
	}
	for _, msg := range e.Errors {
		o.printf("- Lookup error: %s\n", msg)
	}
}

func repoNotes(info *repoInfo) string {
	if info == nil {
		return ""
	}
	var notes []string
	if info.Archived {
		notes = append(notes, "**archived**")
	}
	notes = append(notes, "default branch `"+info.DefaultBranch+"`")
	if info.PushedAt != "" {
		notes = append(notes, "last push "+info.PushedAt[:min(10, len(info.PushedAt))])
	}
	return " (" + strings.Join(notes, ", ") + ")"
}

func wrapCode(items []string) []string {
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = "`" + s + "`"
	}
	return out
}
