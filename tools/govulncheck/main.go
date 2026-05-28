// Command govulncheck is a thin wrapper around golang.org/x/vuln/cmd/govulncheck
// that adds a YAML-configurable ignore list, so CI can stay green on findings
// that have been explicitly reviewed and accepted (e.g. unfixable upstream
// CVEs in a transitively-imported package that we don't actually exercise).
//
// It runs govulncheck once per Go module discovered under the repo root
// (excluding testdata fixtures) and filters the JSON stream against the
// configured ignore list. The exit code is non-zero only if there are
// remaining actionable symbol-level findings.
//
// Usage:
//
//	govulncheck [-config <path>] [-tags <comma-separated>]
//
// govulncheck itself is invoked via `go run` at a pinned version (see the
// govulncheckPkg constant below); the pinned reference is what Renovate
// tracks.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Pinned tool version. Renovate updates the line below.
// renovate: datasource=go packageName=golang.org/x/vuln/cmd/govulncheck
const govulncheckPkg = "golang.org/x/vuln/cmd/govulncheck@v1.3.0"

func main() {
	root := flag.String("root", ".", "repo root to discover Go modules under")
	configPath := flag.String("config", ".govulncheck.yaml", "path to YAML config file with ignore list (optional)")
	tags := flag.String("tags", "", "comma-separated build tags passed to govulncheck (matches -tags flag)")
	flag.Parse()

	if err := run(*root, *configPath, *tags, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "govulncheck:", err)
		os.Exit(2) // 2 = wrapper error (config, exec). 1 = actionable findings.
	}
}

func run(root, configPath, tags string, now time.Time) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	modules, err := discoverModules(root)
	if err != nil {
		return fmt.Errorf("discover modules: %w", err)
	}

	report := newReport(cfg, now)
	for _, mod := range modules {
		result, err := scanModule(mod, tags)
		if err != nil {
			return fmt.Errorf("scan %s: %w", mod, err)
		}
		report.add(mod, result)
	}

	report.print(os.Stdout)
	if report.hasActionable() {
		os.Exit(1)
	}
	return nil
}
