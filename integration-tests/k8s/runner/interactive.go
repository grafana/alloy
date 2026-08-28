package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/charmbracelet/huh"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// configureInteractive opens a TUI for the common runner options. Reuse
// cluster and skip image builds default to selected (typical dev flow).
func configureInteractive(cfg *config) error {
	runOpts := []string{"reuse-cluster", "skip-image-builds"}
	filterMode := "all"
	shard := cfg.shard
	if shard == "" {
		shard = "0/2"
	}

	pkgs, err := discoverTestPackages(cfg.repoRoot)
	if err != nil {
		return fmt.Errorf("discover test packages: %w", err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("no test packages found under integration-tests/k8s/tests/")
	}
	pickedPkgs := []string{pkgs[0]} // default to first so the form is valid

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Run options").
				Options(
					huh.NewOption("Reuse kind cluster if one exists", "reuse-cluster").Selected(true),
					huh.NewOption("Skip image builds (alloy, prom-gen)", "skip-image-builds").Selected(true),
				).
				Value(&runOpts),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which tests do you want to run?").
				Options(
					huh.NewOption("All tests, all packages", "all"),
					huh.NewOption("A shard like CI (e.g. 0/2)", "shard"),
					huh.NewOption("Pick test packages", "packages"),
				).
				Value(&filterMode),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Shard (i/n)").
				Description("Pick the shard of the tests you want to run in (index/total) format. For example, 0/2 or 1/2.").
				Value(&shard).
				Validate(harness.ValidateShard),
		).WithHideFunc(func() bool { return filterMode != "shard" }),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Test packages to run").
				Options(buildPkgOptions(pkgs)...).
				Value(&pickedPkgs).
				Validate(func(s []string) error {
					if len(s) == 0 {
						return fmt.Errorf("pick at least one package")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return filterMode != "packages" }),
	)
	if err := form.Run(); err != nil {
		return err
	}

	cfg.reuseCluster = slices.Contains(runOpts, "reuse-cluster")
	cfg.skipImageBuilds = slices.Contains(runOpts, "skip-image-builds")
	switch filterMode {
	case "all":
		cfg.shard = ""
		cfg.packages = nil
	case "shard":
		cfg.shard = shard
		cfg.packages = nil
	case "packages":
		cfg.shard = ""
		cfg.packages = pickedPkgs
	}
	return nil
}

func discoverTestPackages(repoRoot string) ([]string, error) {
	root := filepath.Join(repoRoot, "integration-tests", "k8s", "tests")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkgs = append(pkgs, "./integration-tests/k8s/tests/"+e.Name())
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

func buildPkgOptions(pkgs []string) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(pkgs))
	for _, p := range pkgs {
		// Display the package's leaf directory; the value carries the full path.
		opts = append(opts, huh.NewOption(filepath.Base(p), p))
	}
	return opts
}
