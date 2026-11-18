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

	"github.com/grafana/alloy/tools/internal/git"
)

const (
	buildTags  = "nodocker"
	runTimeout = "30m"
)

// Failure records a package that could not be built or benchmarked.
type Failure struct {
	ImportPath, Checkout, Stage, Message string
}

type runFlags struct {
	baseRef string
	out     string
	count   int
	bench   string
	all     bool
}

func runCommand() *cobra.Command {
	var f runFlags

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Benchmark HEAD against its merge base with --base-ref and write report.md to --out",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), f)
		},
	}

	cmd.Flags().StringVar(&f.baseRef, "base-ref", "origin/main", "Branch the changes are merged into")
	cmd.Flags().StringVar(&f.out, "out", "", "Output directory")
	cmd.Flags().IntVar(&f.count, "count", 10, "Number of runs per package and checkout")
	cmd.Flags().StringVar(&f.bench, "bench", ".", "Benchmark name regexp")
	cmd.Flags().BoolVar(&f.all, "all", false, "Benchmark every package, not just the affected ones")
	_ = cmd.MarkFlagRequired("out")

	return cmd
}

func run(ctx context.Context, f runFlags) error {
	out, err := filepath.Abs(f.out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	head, err := git.Root()
	if err != nil {
		return err
	}

	mergeBase, err := gitOutput(ctx, head, "merge-base", f.baseRef, "HEAD")
	if err != nil {
		return err
	}
	headSHA, err := gitOutput(ctx, head, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	diff, err := gitOutput(ctx, head, "diff", "--name-only", "--no-renames", mergeBase, "HEAD")
	if err != nil {
		return err
	}

	base, err := os.MkdirTemp("", "benchdiff-base-")
	if err != nil {
		return err
	}
	if _, err := gitOutput(ctx, head, "worktree", "add", "--detach", base, mergeBase); err != nil {
		return err
	}
	defer func() { _, _ = gitOutput(context.Background(), head, "worktree", "remove", "--force", base) }()

	baseTree, err := loadTree(ctx, base)
	if err != nil {
		return fmt.Errorf("listing base packages: %w", err)
	}
	headTree, err := loadTree(ctx, head)
	if err != nil {
		return fmt.Errorf("listing head packages: %w", err)
	}
	pkgs := selectPackages(strings.Fields(diff), baseTree, headTree, f.all)
	for _, p := range pkgs {
		fmt.Fprintf(os.Stderr, "selected %s\n", p.ImportPath)
	}

	failures, err := benchmark(ctx, pkgs, map[string]string{checkoutBase: base, checkoutHead: head}, out, f)
	if err != nil {
		return err
	}

	baseFile, headFile := filepath.Join(out, "base.txt"), filepath.Join(out, "head.txt")
	benchstat, err := exec.CommandContext(ctx, "go", "tool", "benchstat", "base="+baseFile, "head="+headFile).Output()
	if err != nil {
		return fmt.Errorf("running benchstat (run benchdiff with go run -C tools): %w", err)
	}

	in := reportInput{
		packages:  pkgs,
		failures:  failures,
		count:     f.count,
		mergeBase: mergeBase,
		headSHA:   headSHA,
		benchstat: string(benchstat),
	}
	if in.base, err = readResults(baseFile); err != nil {
		return err
	}
	if in.head, err = readResults(headFile); err != nil {
		return err
	}

	for name, data := range map[string]string{"benchstat.txt": in.benchstat, "report.md": renderReport(in)} {
		if err := os.WriteFile(filepath.Join(out, name), []byte(data), 0o644); err != nil {
			return err
		}
	}
	return nil
}

const (
	checkoutBase = "base"
	checkoutHead = "head"
)

type target struct {
	pkg      Package
	checkout string
	dir      string // package directory
	bin      string
	failed   bool
}

// benchmark builds a test binary per package and checkout, then runs them
// count times, alternating base and head so both see the same machine state.
// Results are appended to base.txt and head.txt in out.
func benchmark(ctx context.Context, pkgs []Package, roots map[string]string, out string, f runFlags) ([]Failure, error) {
	results := map[string]io.Writer{}
	for checkout := range roots {
		file, err := os.Create(filepath.Join(out, checkout+".txt"))
		if err != nil {
			return nil, err
		}
		defer file.Close()
		results[checkout] = file
	}
	binDir, err := os.MkdirTemp("", "benchdiff-bin-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(binDir) }()

	var failures []Failure
	fail := func(t *target, stage string, output []byte) {
		t.failed = true
		msg := lastLines(string(output), 20)
		fmt.Fprintf(os.Stderr, "failed to %s %s (%s):\n%s\n", stage, t.pkg.ImportPath, t.checkout, msg)
		failures = append(failures, Failure{ImportPath: t.pkg.ImportPath, Checkout: t.checkout, Stage: stage, Message: msg})
	}

	var pairs [][]*target
	for i, pkg := range pkgs {
		var pair []*target
		for _, checkout := range []string{checkoutBase, checkoutHead} {
			if (checkout == checkoutBase && !pkg.InBase) || (checkout == checkoutHead && !pkg.InHead) {
				continue
			}
			t := &target{
				pkg:      pkg,
				checkout: checkout,
				dir:      filepath.Join(roots[checkout], pkg.Dir),
				bin:      filepath.Join(binDir, fmt.Sprintf("%s-%d.test", checkout, i)),
			}
			fmt.Fprintf(os.Stderr, "building %s (%s)\n", pkg.ImportPath, checkout)
			build := exec.CommandContext(ctx, "go", "test", "-c", "-tags", buildTags, "-o", t.bin, pkg.ImportPath)
			build.Dir = filepath.Join(roots[checkout], pkg.ModuleDir)
			build.Env = append(os.Environ(), "GOWORK=off")
			if output, err := build.CombinedOutput(); err != nil {
				fail(t, "build", output)
			}
			pair = append(pair, t)
		}
		pairs = append(pairs, pair)
	}

	args := []string{"-test.run=^$", "-test.bench=" + f.bench, "-test.benchmem", "-test.count=1", "-test.timeout=" + runTimeout}
	for i := range f.count {
		for _, pair := range pairs {
			if i%2 == 1 && len(pair) == 2 {
				pair = []*target{pair[1], pair[0]}
			}
			for _, t := range pair {
				if t.failed {
					continue
				}
				start := time.Now()
				var stdout, stderr bytes.Buffer
				cmd := exec.CommandContext(ctx, t.bin, args...)
				cmd.Dir = t.dir // like go test, so benchmarks find their testdata
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()
				// Results from before a failure are still valid.
				if _, werr := results[t.checkout].Write(stdout.Bytes()); werr != nil {
					return nil, werr
				}
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if err != nil {
					fail(t, "run", append(stdout.Bytes(), stderr.Bytes()...))
					continue
				}
				fmt.Fprintf(os.Stderr, "[%d/%d] %s (%s) in %s\n", i+1, f.count, t.pkg.ImportPath, t.checkout, time.Since(start).Round(time.Millisecond))
			}
		}
	}
	return failures, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(string(out)), nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}
