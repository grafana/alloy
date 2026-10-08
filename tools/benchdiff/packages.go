package benchdiff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"

	"github.com/grafana/alloy/tools/internal/discover"
)

// Reasons a benchmark package was selected, strongest first.
const (
	ReasonDirect     = "direct"
	ReasonGoMod      = "go.mod"
	ReasonReverseDep = "reverse-dep"
	ReasonAll        = "all"
)

// Selection is the set of benchmark packages to run, as written by the
// packages subcommand and read by the run and report subcommands.
type Selection struct {
	MergeBase string    `json:"merge_base"`
	Packages  []Package `json:"packages"`
}

// Package is a Go package containing benchmarks.
type Package struct {
	ImportPath string `json:"import_path"`
	// ModuleDir is the repository-relative directory of the package's module.
	ModuleDir string `json:"module_dir"`
	// Dir is the repository-relative directory of the package.
	Dir    string `json:"dir"`
	InBase bool   `json:"in_base"`
	InHead bool   `json:"in_head"`
	Reason string `json:"reason"`
}

type packagesFlags struct {
	base      string
	head      string
	mergeBase string
	headRef   string
	tags      string
	all       bool
	out       string
}

func packagesCommand() *cobra.Command {
	var f packagesFlags

	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Select benchmark packages affected by the changes between the merge base and head",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPackages(cmd.Context(), f)
		},
	}

	cmd.Flags().StringVar(&f.base, "base", "", "Path to the checkout of the merge base")
	cmd.Flags().StringVar(&f.head, "head", "", "Path to the checkout of the head")
	cmd.Flags().StringVar(&f.mergeBase, "merge-base", "", "Merge base commit")
	cmd.Flags().StringVar(&f.headRef, "head-ref", "HEAD", "Head commit, resolved in the head checkout")
	cmd.Flags().StringVar(&f.tags, "tags", "nodocker", "Go build tags")
	cmd.Flags().BoolVar(&f.all, "all", false, "Select every benchmark package, regardless of what changed")
	cmd.Flags().StringVar(&f.out, "out", "", "Output JSON file (default: stdout)")
	for _, name := range []string{"base", "head", "merge-base"} {
		_ = cmd.MarkFlagRequired(name)
	}

	return cmd
}

func runPackages(ctx context.Context, f packagesFlags) error {
	changed, err := changedFiles(ctx, f.head, f.mergeBase, f.headRef)
	if err != nil {
		return err
	}

	base, err := loadTree(ctx, f.base, f.tags)
	if err != nil {
		return fmt.Errorf("base: %w", err)
	}
	head, err := loadTree(ctx, f.head, f.tags)
	if err != nil {
		return fmt.Errorf("head: %w", err)
	}

	sel := Selection{
		MergeBase: f.mergeBase,
		Packages:  selectPackages(changed, base, head, f.all),
	}

	for _, p := range sel.Packages {
		fmt.Fprintf(os.Stderr, "selected %s (%s, base=%t, head=%t)\n", p.ImportPath, p.Reason, p.InBase, p.InHead)
	}
	fmt.Fprintf(os.Stderr, "%d changed files, %d benchmark packages selected\n", len(changed), len(sel.Packages))

	return writeJSON(f.out, sel)
}

func changedFiles(ctx context.Context, dir, mergeBase, headRef string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--no-renames", mergeBase, headRef)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing changed files: %w", err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

type module struct {
	// Dir is the repository-relative directory of the module.
	Dir  string
	Path string
}

type benchPackage struct {
	ModuleDir string
	Dir       string
}

// tree describes the benchmark packages of one checkout.
type tree struct {
	modules []module
	// bench maps import paths of packages with benchmarks to their location.
	bench map[string]benchPackage
	// deps maps import paths of packages with benchmarks to the transitive
	// dependencies of their test binary.
	deps map[string][]string
}

func loadTree(ctx context.Context, root, tags string) (tree, error) {
	t := tree{
		bench: map[string]benchPackage{},
		deps:  map[string][]string{},
	}

	var err error
	if t.modules, err = findModules(root); err != nil {
		return t, err
	}

	for _, mod := range t.modules {
		dirs, err := findBenchDirs(root, mod, t.modules)
		if err != nil {
			return t, err
		}
		if len(dirs) == 0 {
			continue
		}
		for _, dir := range dirs {
			t.bench[importPath(mod, dir)] = benchPackage{ModuleDir: mod.Dir, Dir: dir}
		}

		deps, err := listTestDeps(ctx, root, mod, dirs, tags)
		if err != nil {
			return t, err
		}
		for pkg, d := range deps {
			t.deps[pkg] = d
		}
	}

	return t, nil
}

func findModules(root string) ([]module, error) {
	res, err := discover.GoModFiles(root)
	if err != nil {
		return nil, err
	}

	var mods []module
	for _, file := range res.Files() {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		modPath := modfile.ModulePath(data)
		if modPath == "" {
			return nil, fmt.Errorf("%s: missing module path", file)
		}
		rel, err := filepath.Rel(root, filepath.Dir(file))
		if err != nil {
			return nil, err
		}
		mods = append(mods, module{Dir: filepath.ToSlash(rel), Path: modPath})
	}
	return mods, nil
}

var benchFuncRe = regexp.MustCompile(`(?m)^func Benchmark\w*\(`)

// findBenchDirs returns the repository-relative directories of mod that hold
// _test.go files declaring benchmarks. Nested modules are skipped.
func findBenchDirs(root string, mod module, all []module) ([]string, error) {
	nested := map[string]struct{}{}
	for _, m := range all {
		if m.Dir != mod.Dir {
			nested[m.Dir] = struct{}{}
		}
	}

	seen := map[string]struct{}{}
	var dirs []string
	modRoot := filepath.Join(root, filepath.FromSlash(mod.Dir))
	err := filepath.WalkDir(modRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if p != modRoot && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			if _, ok := nested[rel]; ok {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		dir := path.Dir(rel)
		if _, ok := seen[dir]; ok {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if benchFuncRe.Match(data) {
			seen[dir] = struct{}{}
			dirs = append(dirs, dir)
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs, err
}

// relToModule returns dir relative to the module directory, using "." for the
// module directory itself.
func relToModule(mod module, dir string) string {
	switch {
	case dir == mod.Dir:
		return "."
	case mod.Dir == ".":
		return dir
	default:
		return strings.TrimPrefix(dir, mod.Dir+"/")
	}
}

// skipDir reports whether the go tool ignores directories named name.
func skipDir(name string) bool {
	switch name {
	case "testdata", "vendor", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func importPath(mod module, dir string) string {
	if rel := relToModule(mod, dir); rel != "." {
		return mod.Path + "/" + rel
	}
	return mod.Path
}

type listedPackage struct {
	ImportPath string
	Name       string
	Deps       []string
}

// listTestDeps returns, for every package in dirs, the transitive
// dependencies of its test binary (including internal and external tests).
func listTestDeps(ctx context.Context, root string, mod module, dirs []string, tags string) (map[string][]string, error) {
	args := []string{"list", "-e", "-test", "-json=ImportPath,Name,Deps"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	for _, dir := range dirs {
		args = append(args, "./"+relToModule(mod, dir))
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = filepath.Join(root, filepath.FromSlash(mod.Dir))
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list in %s: %w: %s", mod.Dir, err, stderr.String())
	}

	deps := map[string][]string{}
	dec := json.NewDecoder(&stdout)
	for {
		var p listedPackage
		if err := dec.Decode(&p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding go list output: %w", err)
		}
		// The generated test main package ("pkg.test") depends on both the
		// internal and external test variants, so its Deps cover everything
		// the benchmarks can exercise.
		if p.Name != "main" || !strings.HasSuffix(p.ImportPath, ".test") {
			continue
		}
		pkg := strings.TrimSuffix(p.ImportPath, ".test")
		for _, d := range p.Deps {
			// Test variants are listed as "pkg [pkg.test]".
			if i := strings.Index(d, " ["); i >= 0 {
				d = d[:i]
			}
			deps[pkg] = append(deps[pkg], d)
		}
	}
	return deps, nil
}

// selectPackages returns the benchmark packages, from either tree, that are
// affected by the changed files.
func selectPackages(changed []string, base, head tree, all bool) []Package {
	changedPkgs := map[string]struct{}{}
	changedMods := map[string]struct{}{}
	for _, file := range changed {
		switch path.Base(file) {
		case "go.mod", "go.sum":
			changedMods[path.Dir(file)] = struct{}{}
			continue
		}
		if !isSourceFile(file) {
			continue
		}
		for _, t := range []tree{base, head} {
			if pkg, ok := owningPackage(file, t.modules); ok {
				changedPkgs[pkg] = struct{}{}
			}
		}
	}

	reasonFor := func(t tree, pkg string, bp benchPackage) string {
		if all {
			return ReasonAll
		}
		for _, file := range changed {
			if path.Dir(file) == bp.Dir || strings.HasPrefix(file, bp.Dir+"/testdata/") {
				return ReasonDirect
			}
		}
		if _, ok := changedMods[bp.ModuleDir]; ok {
			return ReasonGoMod
		}
		for _, dep := range t.deps[pkg] {
			if _, ok := changedPkgs[dep]; ok {
				return ReasonReverseDep
			}
		}
		return ""
	}

	selected := map[string]*Package{}
	for _, t := range []tree{base, head} {
		for pkg, bp := range t.bench {
			reason := reasonFor(t, pkg, bp)
			if reason == "" {
				continue
			}
			p, ok := selected[pkg]
			if !ok {
				_, inBase := base.bench[pkg]
				_, inHead := head.bench[pkg]
				p = &Package{
					ImportPath: pkg,
					ModuleDir:  bp.ModuleDir,
					Dir:        bp.Dir,
					InBase:     inBase,
					InHead:     inHead,
					Reason:     reason,
				}
				selected[pkg] = p
			}
			if reasonRank(reason) < reasonRank(p.Reason) {
				p.Reason = reason
			}
		}
	}

	pkgs := make([]Package, 0, len(selected))
	for _, p := range selected {
		pkgs = append(pkgs, *p)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })
	return pkgs
}

func reasonRank(reason string) int {
	switch reason {
	case ReasonAll:
		return 0
	case ReasonDirect:
		return 1
	case ReasonGoMod:
		return 2
	default:
		return 3
	}
}

// isSourceFile reports whether a change to file can affect the packages that
// import its package. Test files only affect their own package.
func isSourceFile(file string) bool {
	if strings.HasSuffix(file, "_test.go") {
		return false
	}
	switch path.Ext(file) {
	case ".go", ".s", ".c", ".h":
		return true
	}
	return false
}

// owningPackage returns the import path of the package containing file, based
// on the innermost module containing it.
func owningPackage(file string, mods []module) (string, bool) {
	dir := path.Dir(file)
	best, bestLen := -1, -1
	for i, m := range mods {
		n := 0
		if m.Dir != "." {
			if dir != m.Dir && !strings.HasPrefix(dir, m.Dir+"/") {
				continue
			}
			n = len(m.Dir)
		}
		if n > bestLen {
			best, bestLen = i, n
		}
	}
	if best < 0 {
		return "", false
	}
	return importPath(mods[best], dir), true
}

func writeJSON(file string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if file == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(file, data, 0o644)
}

func readJSON(file string, v any) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
