// Package govulncheck runs golang.org/x/vuln/cmd/govulncheck across every Go
// module in the repo and applies a YAML-configurable ignore list, so CI can
// stay green on reviewed-and-accepted findings. govulncheck's native text
// output is streamed through unchanged; we only post-process it to decide
// the exit code. See .govulncheck.yaml for the ignore schema.
package govulncheck

import (
	"bytes"
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

	"github.com/spf13/cobra"
)

// renovate: datasource=go packageName=golang.org/x/vuln/cmd/govulncheck
const govulncheckPkg = "golang.org/x/vuln/cmd/govulncheck@v1.3.0"

// Command returns the cobra command for tools/cmd to register.
func Command() *cobra.Command {
	var root, configPath, tags string
	cmd := &cobra.Command{
		Use:   "govulncheck",
		Short: "Run govulncheck across every Go module and apply the YAML ignore list",
		RunE: func(cmd *cobra.Command, args []string) error {
			actionable, err := run(root, configPath, tags, time.Now())
			if err != nil {
				return err
			}
			if actionable {
				// Use SilenceUsage/SilenceErrors so cobra doesn't print
				// usage on a "vulnerabilities found" exit — the findings
				// themselves are already printed by govulncheck above.
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "repo root to discover Go modules under")
	cmd.Flags().StringVar(&configPath, "config", ".govulncheck.yaml", "path to YAML ignore-list config (optional)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated build tags passed through to govulncheck")
	return cmd
}

// run scans every discovered module and returns whether any actionable
// (non-ignored) findings remain.
func run(root, configPath, tags string, now time.Time) (bool, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}
	modules, err := discoverModules(root)
	if err != nil {
		return false, fmt.Errorf("discover modules: %w", err)
	}

	var allActionable, allIgnored []string
	for _, mod := range modules {
		fmt.Printf("\n==> govulncheck %s\n", mod)
		out, gerr := scan(mod, tags)
		ids := parseSymbolFindings(out)

		// Non-zero exit with zero parsed IDs means either a real tool
		// error or that the text format changed under us — never silently
		// let findings through.
		if gerr != nil && len(ids) == 0 {
			return false, fmt.Errorf("%s: govulncheck failed (%v) and parser found no Symbol findings — tool error or output format changed", mod, gerr)
		}

		actionable, ignored := classify(ids, cfg, now)
		allActionable = append(allActionable, actionable...)
		allIgnored = append(allIgnored, ignored...)
	}

	printFilterReport(os.Stdout, cfg, dedup(allActionable), dedup(allIgnored), len(modules), now)
	return len(allActionable) > 0, nil
}

// scan runs govulncheck in dir, tee-ing its native text output to stdout and
// to a buffer. The returned error is non-nil on findings or tool failure.
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

// e.g. `Vulnerability #3: GO-2026-5018`
var vulnHeaderRe = regexp.MustCompile(`^Vulnerability #\d+: (GO-\d{4}-\d+)$`)

// parseSymbolFindings returns the reachable OSV IDs reported by govulncheck.
// Only the `=== Symbol Results ===` section counts as reachable; Package and
// Module sections are informational (govulncheck itself exits 0 for those).
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

// printFilterReport writes a per-status summary after all govulncheck output,
// listing each unique ID once with its URL (actionable) or reason (ignored).
func printFilterReport(w io.Writer, cfg *Config, actionable, ignored []string, modules int, now time.Time) {
	fmt.Fprintf(w, "\nwrapper summary across %d modules\n", modules)
	if len(actionable) == 0 && len(ignored) == 0 {
		fmt.Fprintln(w, "  no reachable vulnerabilities")
		return
	}
	for _, id := range ignored {
		// Invariant: id came from classify() with this same now, so isIgnored returns non-nil.
		fmt.Fprintf(w, "  [IGN]  %s  %s\n", id, oneLine(cfg.isIgnored(id, now).Reason))
	}
	for _, id := range actionable {
		fmt.Fprintf(w, "  [FAIL] %s  https://pkg.go.dev/vuln/%s\n", id, id)
	}
	fmt.Fprintf(w, "  → %d actionable, %d ignored\n", len(actionable), len(ignored))
}

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

// discoverModules returns Go module directories under root, excluding
// testdata fixtures.
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
