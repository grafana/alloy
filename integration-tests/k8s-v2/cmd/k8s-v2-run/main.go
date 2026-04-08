package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/planner"
)

const (
	testsRoot  = "integration-tests/k8s-v2/tests"
	packageDir = "./integration-tests/k8s-v2"
)

type options struct {
	tests       string
	keepCluster bool
	verbose     bool
	timeout     time.Duration
	listOnly    bool
}

func main() {
	opts, passthrough, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		printUsage()
		os.Exit(2)
	}

	allTests, err := planner.DiscoverTests(testsRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discover k8s-v2 tests: %v\n", err)
		os.Exit(1)
	}

	if opts.listOnly {
		printTests(allTests)
		return
	}

	resolvedTests, err := resolveSelection(opts.tests, allTests)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	args := buildGoTestArgs(opts, resolvedTests, passthrough)
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "run go test: %v\n", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	opts.tests = "all"
	opts.timeout = 30 * time.Minute

	var passthrough []string
	for i, arg := range args {
		if arg == "--" {
			passthrough = args[i+1:]
			args = args[:i]
			break
		}
	}

	fs := flag.NewFlagSet("k8s-v2-run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.tests, "tests", "all", "Comma-separated tests or aliases (all, metrics, logs)")
	fs.BoolVar(&opts.keepCluster, "keep-cluster", false, "Keep KinD cluster after run")
	fs.BoolVar(&opts.verbose, "v", true, "Enable verbose go test output")
	fs.DurationVar(&opts.timeout, "timeout", 30*time.Minute, "go test timeout")
	fs.BoolVar(&opts.listOnly, "list", false, "List discovered tests and exit")

	if err := fs.Parse(args); err != nil {
		return options{}, nil, err
	}
	return opts, passthrough, nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run ./integration-tests/k8s-v2/cmd/k8s-v2-run [flags] [-- <extra go test args>]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./integration-tests/k8s-v2/cmd/k8s-v2-run --list")
	fmt.Println("  go run ./integration-tests/k8s-v2/cmd/k8s-v2-run --tests metrics")
	fmt.Println("  go run ./integration-tests/k8s-v2/cmd/k8s-v2-run --tests metrics,logs")
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
	fmt.Println("aliases: metrics -> metrics-mimir, logs -> logs-loki")
}

func resolveSelection(raw string, all []planner.TestCase) (string, error) {
	if raw == "" || raw == "all" {
		return "all", nil
	}

	known := make(map[string]struct{}, len(all))
	names := make([]string, 0, len(all))
	for _, tc := range all {
		known[tc.Name] = struct{}{}
		names = append(names, tc.Name)
	}

	aliases := map[string]string{
		"metrics": "metrics-mimir",
		"logs":    "logs-loki",
	}

	var resolved []string
	seen := map[string]struct{}{}
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		token = strings.ToLower(token)
		if mapped, ok := aliases[token]; ok {
			token = mapped
		}

		if _, ok := known[token]; !ok {
			matches := prefixMatches(token, names)
			if len(matches) == 1 {
				token = matches[0]
			} else if len(matches) > 1 {
				return "", fmt.Errorf("ambiguous test selector %q matches %v", token, matches)
			} else {
				return "", fmt.Errorf("unknown test selector %q", token)
			}
		}

		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		resolved = append(resolved, token)
	}

	if len(resolved) == 0 {
		return "", fmt.Errorf("no tests selected")
	}
	return strings.Join(resolved, ","), nil
}

func prefixMatches(prefix string, names []string) []string {
	var out []string
	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func buildGoTestArgs(opts options, selected string, passthrough []string) []string {
	args := []string{"test"}
	if opts.verbose {
		args = append(args, "-v")
	}
	args = append(args, "-timeout", opts.timeout.String())
	args = append(args, packageDir)
	args = append(args, passthrough...)
	args = append(args, "-args", "-k8s.v2.tests="+selected)
	if opts.keepCluster {
		args = append(args, "-k8s.v2.keep-cluster=true")
	}
	return args
}
