package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/planner"
	"github.com/spf13/cobra"
)

const (
	testsRoot         = "integration-tests/k8s-v2/tests"
	packageDir        = "./integration-tests/k8s-v2"
	integrationGoTags = "alloyintegrationtests k8sv2integrationtests"
)

type options struct {
	all              bool
	tests            []string
	keepCluster      bool
	keepDeps         bool
	reuseCluster     string
	reuseDeps        bool
	verbose          bool
	debug            bool
	timeout          time.Duration
	setupTimeout     time.Duration
	readinessTimeout time.Duration
}

type selectedTest struct {
	Name    string
	AbsPath string
}

func main() {
	opts := options{
		verbose:          true,
		timeout:          30 * time.Minute,
		setupTimeout:     20 * time.Minute,
		readinessTimeout: 2 * time.Minute,
	}

	rootCmd := &cobra.Command{
		Use:   "runner [-- <extra go test args>]",
		Short: "Run k8s-v2 integration tests",
		Long: "Runs k8s-v2 integration tests using test folder names from integration-tests/k8s-v2/tests.\n\n" +
			"Examples:\n" +
			"  go run ./integration-tests/k8s-v2/runner --all\n" +
			"  go run ./integration-tests/k8s-v2/runner --test metrics-mimir --test logs-loki\n" +
			"  go run ./integration-tests/k8s-v2/runner --test integration-tests/k8s-v2/tests/logs-loki\n" +
			"  go run ./integration-tests/k8s-v2/runner --test metrics-mimir --keep-cluster --keep-deps --debug\n" +
			"  go run ./integration-tests/k8s-v2/runner --test metrics-mimir --reuse-cluster alloy-k8s-v2-dev --reuse-deps\n" +
			"  go run ./integration-tests/k8s-v2/runner --test metrics-mimir -- --count=1",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, passthrough []string) error {
			return run(opts, passthrough)
		},
	}

	rootCmd.Flags().BoolVar(&opts.all, "all", false, "Run all discovered tests")
	rootCmd.Flags().StringArrayVar(&opts.tests, "test", nil, "Test name or test folder path; can be specified multiple times")
	rootCmd.Flags().BoolVar(&opts.keepCluster, "keep-cluster", false, "Keep KinD cluster after run")
	rootCmd.Flags().BoolVar(&opts.keepDeps, "keep-deps", false, "Keep installed dependencies after run (requires --keep-cluster)")
	rootCmd.Flags().StringVar(&opts.reuseCluster, "reuse-cluster", "", "Reuse an existing Kind cluster by name")
	rootCmd.Flags().BoolVar(&opts.reuseDeps, "reuse-deps", false, "When reusing a cluster, skip dependency install/uninstall checks")
	rootCmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", true, "Enable verbose go test output")
	rootCmd.Flags().BoolVar(&opts.debug, "debug", false, "Enable debug logging for setup and dependency checks")
	rootCmd.Flags().DurationVar(&opts.timeout, "timeout", 30*time.Minute, "go test timeout")
	rootCmd.Flags().DurationVar(&opts.setupTimeout, "setup-timeout", 20*time.Minute, "setup timeout for cluster creation and dependency install")
	rootCmd.Flags().DurationVar(&opts.readinessTimeout, "readiness-timeout", 2*time.Minute, "readiness timeout for dependency checks")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List discovered test names",
		RunE: func(cmd *cobra.Command, args []string) error {
			allTests, err := planner.DiscoverTests(testsRoot)
			if err != nil {
				return fmt.Errorf("discover k8s-v2 tests: %w", err)
			}
			printTests(allTests)
			return nil
		},
	}

	rootCmd.AddCommand(listCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts options, passthrough []string) error {
	if opts.keepDeps && !opts.keepCluster {
		return fmt.Errorf("--keep-deps requires --keep-cluster")
	}
	if opts.reuseDeps && opts.reuseCluster == "" {
		return fmt.Errorf("--reuse-deps requires --reuse-cluster")
	}

	allTests, err := planner.DiscoverTests(testsRoot)
	if err != nil {
		return fmt.Errorf("discover k8s-v2 tests: %w", err)
	}

	resolvedTests, err := resolveSelection(opts, allTests)
	if err != nil {
		return err
	}

	fmt.Println("Resolved tests:")
	for _, test := range resolvedTests {
		fmt.Printf("  %s -> %s\n", test.Name, test.AbsPath)
	}

	args := buildGoTestArgs(opts, resolvedTests, passthrough)
	fmt.Printf("Executing: go %s\n", strings.Join(args, " "))

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func printTests(all []planner.TestCase) {
	names := make([]string, 0, len(all))
	for _, tc := range all {
		names = append(names, tc.Name)
	}
	sort.Strings(names)

	fmt.Println("k8s-v2 discovered tests:")
	fmt.Println("  all")
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}
}

func resolveSelection(opts options, all []planner.TestCase) ([]selectedTest, error) {
	if opts.all && len(opts.tests) > 0 {
		return nil, fmt.Errorf("use either --all or --test, not both")
	}
	if !opts.all && len(opts.tests) == 0 {
		return nil, fmt.Errorf("select tests with --all or at least one --test")
	}

	knownByName := make(map[string]planner.TestCase, len(all))
	knownByLowerName := make(map[string]planner.TestCase, len(all))
	names := make([]string, 0, len(all))
	absDirToName := make(map[string]string, len(all))
	for _, tc := range all {
		knownByName[tc.Name] = tc
		knownByLowerName[strings.ToLower(tc.Name)] = tc
		names = append(names, tc.Name)
		absDir, err := filepath.Abs(tc.Dir)
		if err == nil {
			absDirToName[filepath.Clean(absDir)] = tc.Name
		}
	}
	sort.Strings(names)

	if opts.all {
		out := make([]selectedTest, 0, len(names))
		for _, n := range names {
			tc := knownByName[n]
			absPath, err := filepath.Abs(tc.Dir)
			if err != nil {
				return nil, fmt.Errorf("resolve absolute path for %q: %w", tc.Name, err)
			}
			out = append(out, selectedTest{Name: tc.Name, AbsPath: filepath.Clean(absPath)})
		}
		return out, nil
	}

	var resolved []selectedTest
	seen := map[string]struct{}{}
	for _, token := range opts.tests {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		selected, err := resolveSingleTest(token, knownByName, knownByLowerName, absDirToName)
		if err != nil {
			return nil, fmt.Errorf("%v (valid test names: %s)", err, strings.Join(names, ", "))
		}

		if _, ok := seen[selected.Name]; ok {
			continue
		}
		seen[selected.Name] = struct{}{}
		resolved = append(resolved, selected)
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("no tests selected")
	}
	return resolved, nil
}

func resolveSingleTest(
	raw string,
	knownByName map[string]planner.TestCase,
	knownByLowerName map[string]planner.TestCase,
	absDirToName map[string]string,
) (selectedTest, error) {

	if tc, ok := knownByName[raw]; ok {
		absPath, err := filepath.Abs(tc.Dir)
		if err != nil {
			return selectedTest{}, fmt.Errorf("resolve absolute path for %q: %w", tc.Name, err)
		}
		return selectedTest{Name: tc.Name, AbsPath: filepath.Clean(absPath)}, nil
	}
	if tc, ok := knownByLowerName[strings.ToLower(raw)]; ok {
		absPath, err := filepath.Abs(tc.Dir)
		if err != nil {
			return selectedTest{}, fmt.Errorf("resolve absolute path for %q: %w", tc.Name, err)
		}
		return selectedTest{Name: tc.Name, AbsPath: filepath.Clean(absPath)}, nil
	}
	return resolveTestPath(raw, absDirToName)
}

func resolveTestPath(raw string, absDirToName map[string]string) (selectedTest, error) {
	expanded := os.ExpandEnv(raw)
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return selectedTest{}, fmt.Errorf("resolve home directory for %q: %w", raw, err)
		}
		if expanded == "~" {
			expanded = homeDir
		} else {
			expanded = filepath.Join(homeDir, strings.TrimPrefix(expanded, "~/"))
		}
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return selectedTest{}, fmt.Errorf("resolve test path %q: %w", raw, err)
	}
	cleanAbs := filepath.Clean(abs)

	info, statErr := os.Stat(cleanAbs)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return selectedTest{}, fmt.Errorf("test path %q does not exist", raw)
		}
		return selectedTest{}, fmt.Errorf("stat test path %q: %w", raw, statErr)
	}
	if !info.IsDir() {
		return selectedTest{}, fmt.Errorf("test path %q is not a directory", raw)
	}

	if name, ok := absDirToName[cleanAbs]; ok {
		return selectedTest{Name: name, AbsPath: cleanAbs}, nil
	}

	return selectedTest{}, fmt.Errorf("test path %q is not a known k8s-v2 test folder", raw)
}

func buildGoTestArgs(opts options, selected []selectedTest, passthrough []string) []string {
	selectedNames := make([]string, 0, len(selected))
	for _, s := range selected {
		selectedNames = append(selectedNames, s.Name)
	}

	args := []string{"test"}
	if opts.verbose {
		args = append(args, "-v")
	}
	args = append(args, "-tags", integrationGoTags)
	args = append(args, "-timeout", opts.timeout.String())
	args = append(args, packageDir)
	args = append(args, passthrough...)
	args = append(
		args,
		"-args",
		"-k8s.v2.tests="+strings.Join(selectedNames, ","),
		"-k8s.v2.setup-timeout="+opts.setupTimeout.String(),
		"-k8s.v2.readiness-timeout="+opts.readinessTimeout.String(),
	)
	if opts.keepCluster {
		args = append(args, "-k8s.v2.keep-cluster=true")
	}
	if opts.keepDeps {
		args = append(args, "-k8s.v2.keep-deps=true")
	}
	if opts.reuseCluster != "" {
		args = append(args, "-k8s.v2.reuse-cluster="+opts.reuseCluster)
	}
	if opts.reuseDeps {
		args = append(args, "-k8s.v2.reuse-deps=true")
	}
	if opts.debug {
		args = append(args, "-k8s.v2.debug=true")
	}
	return args
}
