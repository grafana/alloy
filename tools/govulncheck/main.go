// Command govulncheck is a thin wrapper around golang.org/x/vuln/cmd/govulncheck
// that adds a YAML-configurable ignore list, so CI can stay green on findings
// that have been explicitly reviewed and accepted (e.g. unfixable upstream
// CVEs in a transitively-imported package that's not actually exercised).
//
// govulncheck's native text output is streamed to stdout for clean CI logs;
// the wrapper only post-processes it to extract reachable OSV IDs from the
// "=== Symbol Results ===" section and apply the ignore list. If the tool
// reports findings but the parser sees none, we fail loudly rather than
// silently letting findings through — this catches govulncheck output format
// changes between releases.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Pinned tool version. Renovate updates the line below.
// renovate: datasource=go packageName=golang.org/x/vuln/cmd/govulncheck
const govulncheckPkg = "golang.org/x/vuln/cmd/govulncheck@v1.3.0"

func main() {
	root := flag.String("root", ".", "repo root to discover Go modules under")
	configPath := flag.String("config", ".govulncheck.yaml", "path to YAML ignore-list config (optional)")
	tags := flag.String("tags", "", "comma-separated build tags passed through to govulncheck")
	flag.Parse()

	exitCode, err := run(*root, *configPath, *tags, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "govulncheck wrapper:", err)
		os.Exit(2)
	}
	os.Exit(exitCode)
}

func run(root, configPath, tags string, now time.Time) (int, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return 0, fmt.Errorf("load config: %w", err)
	}
	modules, err := discoverModules(root)
	if err != nil {
		return 0, fmt.Errorf("discover modules: %w", err)
	}

	var allActionable, allIgnored []string
	for _, mod := range modules {
		fmt.Printf("\n==> govulncheck %s\n", mod)
		out, gerr := scan(mod, tags)
		ids := parseSymbolFindings(out)

		// Defensive: govulncheck exits non-zero when it finds reachable
		// vulns. If it failed but our parser extracted nothing from the
		// Symbol Results section, either the tool errored (e.g. couldn't
		// fetch deps) or its text format has changed in a way we don't
		// recognise. Either way, fail loud — never silently let findings
		// through.
		if gerr != nil && len(ids) == 0 {
			return 0, fmt.Errorf("%s: govulncheck failed (%v) and parser found no Symbol findings — tool error or output format changed", mod, gerr)
		}

		actionable, ignored := classify(ids, cfg, now)
		allActionable = append(allActionable, actionable...)
		allIgnored = append(allIgnored, ignored...)
	}

	printFilterReport(os.Stdout, cfg, dedup(allActionable), dedup(allIgnored), len(modules))
	if len(allActionable) > 0 {
		return 1, nil
	}
	return 0, nil
}

// scan runs govulncheck in dir, streaming its native text output to stdout
// while also capturing it for parsing. Returns the captured output and the
// command's exit error (nil on clean exit, non-nil on findings or tool error).
func scan(dir, tags string) (string, error) {
	args := []string{"run", govulncheckPkg}
	if tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, "./...")
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	err := cmd.Run()
	return buf.String(), err
}

// vulnHeaderRe matches the per-vulnerability header line that govulncheck
// emits inside each results section, e.g. `Vulnerability #3: GO-2026-5018`.
var vulnHeaderRe = regexp.MustCompile(`^Vulnerability #\d+: (GO-\d{4}-\d+)$`)

// parseSymbolFindings returns the set of reachable OSV IDs from govulncheck's
// text output. Only the `=== Symbol Results ===` section is considered:
// Package and Module sections list deps with vulns whose vulnerable symbols
// are not actually called from our code, which govulncheck itself flags as
// informational only (and which match govulncheck's own exit-0 behaviour).
func parseSymbolFindings(out string) []string {
	const symbolHeader = "=== Symbol Results ==="
	const sectionPrefix = "=== "

	var found []string
	inSymbol := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, symbolHeader):
			inSymbol = true
		case strings.HasPrefix(line, sectionPrefix):
			inSymbol = false
		case inSymbol:
			if m := vulnHeaderRe.FindStringSubmatch(line); m != nil {
				found = append(found, m[1])
			}
		}
	}
	return found
}

// classify splits ids into actionable (not ignored) and ignored buckets,
// honouring the config's per-entry expiry.
func classify(ids []string, cfg *Config, now time.Time) (actionable, ignored []string) {
	for _, id := range ids {
		if cfg.isIgnored(id, now) != nil {
			ignored = append(ignored, id)
		} else {
			actionable = append(actionable, id)
		}
	}
	return actionable, ignored
}

// printFilterReport writes a per-status summary at the very end of the run,
// after all govulncheck native output. Each unique ID is listed once with
// the upstream URL (actionable) or the ignore reason (ignored).
func printFilterReport(w io.Writer, cfg *Config, actionable, ignored []string, modules int) {
	fmt.Fprintf(w, "\nwrapper summary across %d modules\n", modules)
	if len(actionable) == 0 && len(ignored) == 0 {
		fmt.Fprintln(w, "  no reachable vulnerabilities")
		return
	}
	for _, id := range ignored {
		reason := "(missing reason)"
		if e := cfg.isIgnored(id, time.Now()); e != nil {
			reason = oneLine(e.Reason)
		}
		fmt.Fprintf(w, "  [IGN]  %s  %s\n", id, reason)
	}
	for _, id := range actionable {
		fmt.Fprintf(w, "  [FAIL] %s  https://pkg.go.dev/vuln/%s\n", id, id)
	}
	fmt.Fprintf(w, "  → %d actionable, %d ignored\n", len(actionable), len(ignored))
}

// oneLine collapses a multi-line ignore reason to a single line for the
// summary; the full reason still lives in the YAML for human readers.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// discoverModules returns Go module directories under root, excluding any
// path containing a testdata segment.
func discoverModules(root string) ([]string, error) {
	var modules []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		dir := filepath.Dir(path)
		for _, part := range strings.Split(filepath.ToSlash(dir), "/") {
			if part == "testdata" {
				return nil
			}
		}
		modules = append(modules, dir)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(modules)
	return modules, nil
}
