package benchdiff

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	TreeBase = "base"
	TreeHead = "head"
)

// Meta describes a benchmark run, as written by the run subcommand.
type Meta struct {
	Count    int       `json:"count"`
	Failures []Failure `json:"failures,omitempty"`
}

// Failure records a package that could not be built or benchmarked.
type Failure struct {
	ImportPath string `json:"import_path"`
	Tree       string `json:"tree"`
	Stage      string `json:"stage"`
	Message    string `json:"message"`
}

type runFlags struct {
	base      string
	head      string
	packages  string
	out       string
	tags      string
	bench     string
	benchtime string
	timeout   string
	count     int
}

func runCommand() *cobra.Command {
	var f runFlags

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the selected benchmarks on both checkouts, interleaving base and head runs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBenchmarks(cmd.Context(), f)
		},
	}

	cmd.Flags().StringVar(&f.base, "base", "", "Path to the checkout of the merge base")
	cmd.Flags().StringVar(&f.head, "head", "", "Path to the checkout of the head")
	cmd.Flags().StringVar(&f.packages, "packages", "", "Packages JSON file written by the packages subcommand")
	cmd.Flags().StringVar(&f.out, "out", "", "Output directory for base.txt, head.txt and meta.json")
	cmd.Flags().StringVar(&f.tags, "tags", "nodocker", "Go build tags")
	cmd.Flags().StringVar(&f.bench, "bench", ".", "Benchmark name regexp")
	cmd.Flags().StringVar(&f.benchtime, "benchtime", "", "Value for -test.benchtime (default: Go's default)")
	cmd.Flags().StringVar(&f.timeout, "timeout", "30m", "Timeout for a single run of a package's benchmarks")
	cmd.Flags().IntVar(&f.count, "count", 10, "Number of times to run each package's benchmarks per checkout")
	for _, name := range []string{"base", "head", "packages", "out"} {
		_ = cmd.MarkFlagRequired(name)
	}

	return cmd
}

type target struct {
	pkg  Package
	tree string
	root string
	bin  string
	// failed is set once the target failed to build or run; it is skipped
	// from then on.
	failed bool
}

func runBenchmarks(ctx context.Context, f runFlags) error {
	var sel Selection
	if err := readJSON(f.packages, &sel); err != nil {
		return fmt.Errorf("reading packages: %w", err)
	}

	out, err := filepath.Abs(f.out)
	if err != nil {
		return err
	}
	roots := map[string]string{}
	for tree, dir := range map[string]string{TreeBase: f.base, TreeHead: f.head} {
		if roots[tree], err = filepath.Abs(dir); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(out, "bin", tree), 0o755); err != nil {
			return err
		}
	}

	results := map[string]*os.File{}
	for _, tree := range []string{TreeBase, TreeHead} {
		file, err := os.Create(filepath.Join(out, tree+".txt"))
		if err != nil {
			return err
		}
		defer file.Close()
		results[tree] = file
	}

	meta := Meta{Count: f.count}
	fail := func(t *target, stage, msg string) {
		t.failed = true
		fmt.Fprintf(os.Stderr, "FAILED to %s %s (%s): %s\n", stage, t.pkg.ImportPath, t.tree, msg)
		meta.Failures = append(meta.Failures, Failure{
			ImportPath: t.pkg.ImportPath,
			Tree:       t.tree,
			Stage:      stage,
			Message:    lastLines(msg, 20),
		})
	}

	// Build every test binary up front so that the measurements aren't
	// interleaved with compilation.
	var targets [][]*target
	for _, pkg := range sel.Packages {
		var pair []*target
		for _, tree := range []string{TreeBase, TreeHead} {
			if (tree == TreeBase && !pkg.InBase) || (tree == TreeHead && !pkg.InHead) {
				continue
			}
			t := &target{
				pkg:  pkg,
				tree: tree,
				root: roots[tree],
				bin:  filepath.Join(out, "bin", tree, binaryName(pkg.ImportPath)),
			}
			fmt.Fprintf(os.Stderr, "building %s (%s)\n", pkg.ImportPath, tree)
			if msg, err := buildTestBinary(ctx, t, f.tags); err != nil {
				fail(t, "build", msg)
			}
			pair = append(pair, t)
		}
		targets = append(targets, pair)
	}

	args := []string{"-test.run=^$", "-test.bench=" + f.bench, "-test.benchmem", "-test.count=1", "-test.timeout=" + f.timeout}
	if f.benchtime != "" {
		args = append(args, "-test.benchtime="+f.benchtime)
	}

	for i := range f.count {
		for _, pair := range targets {
			// Alternate which checkout goes first so that neither side is
			// systematically favoured by warm caches or thermal state.
			order := pair
			if i%2 == 1 && len(pair) == 2 {
				order = []*target{pair[1], pair[0]}
			}
			for _, t := range order {
				if t.failed {
					continue
				}
				start := time.Now()
				msg, err := runTestBinary(ctx, t, args, results[t.tree])
				if err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					fail(t, "run", msg)
					continue
				}
				fmt.Fprintf(os.Stderr, "[%d/%d] %s (%s) in %s\n", i+1, f.count, t.pkg.ImportPath, t.tree, time.Since(start).Round(time.Millisecond))
			}
		}
	}

	return writeJSON(filepath.Join(out, "meta.json"), meta)
}

func buildTestBinary(ctx context.Context, t *target, tags string) (string, error) {
	args := []string{"test", "-c", "-o", t.bin}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	mod := module{Dir: t.pkg.ModuleDir}
	args = append(args, "./"+relToModule(mod, t.pkg.Dir))

	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = filepath.Join(t.root, filepath.FromSlash(t.pkg.ModuleDir))
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return output.String(), err
	}
	if _, err := os.Stat(t.bin); err != nil {
		return "no test binary produced: " + output.String(), err
	}
	return "", nil
}

func runTestBinary(ctx context.Context, t *target, args []string, results io.Writer) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, t.bin, args...)
	// Run from the package directory, like go test does, so benchmarks can
	// find their testdata.
	cmd.Dir = filepath.Join(t.root, filepath.FromSlash(t.pkg.Dir))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	// Keep the output of a failed run too: benchmarks that completed before
	// the failure are still valid results.
	if _, werr := results.Write(stdout.Bytes()); werr != nil {
		return "", werr
	}
	if err != nil {
		return stdout.String() + stderr.String(), err
	}
	return "", nil
}

func binaryName(importPath string) string {
	return strings.NewReplacer("/", "_", ".", "_").Replace(importPath) + ".test"
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
