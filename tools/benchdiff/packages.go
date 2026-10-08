package benchdiff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/grafana/alloy/tools/internal/discover"
)

// Package is a Go package with benchmarks.
type Package struct {
	ImportPath string
	ModuleDir  string // relative to the repository root
	Dir        string // relative to the repository root
	InBase     bool
	InHead     bool
}

type benchPackage struct {
	moduleDir, dir string
	deps           []string // dependencies of the package's test binary
}

// tree lists the packages of one checkout of the repository.
type tree struct {
	importPaths map[string]string // directory -> import path
	bench       map[string]benchPackage
}

func loadTree(ctx context.Context, root string) (tree, error) {
	t := tree{importPaths: map[string]string{}, bench: map[string]benchPackage{}}

	mods, err := discover.GoModFiles(root)
	if err != nil {
		return t, err
	}
	for _, dir := range mods.Dirs() {
		if err := t.loadModule(ctx, root, dir); err != nil {
			return t, err
		}
	}
	return t, nil
}

var benchmarkFunc = regexp.MustCompile(`(?m)^func Benchmark\w*\(`)

func (t tree) loadModule(ctx context.Context, root, modDir string) error {
	type listed struct {
		ImportPath, Dir           string
		TestGoFiles, XTestGoFiles []string
	}
	// -find skips resolving dependencies, so this is fast and needs no downloads.
	pkgs, err := goList[listed](ctx, modDir, "-find", "-json=ImportPath,Dir,TestGoFiles,XTestGoFiles", "./...")
	if err != nil {
		return err
	}

	moduleDir := relPath(root, modDir)
	var benchPkgs []string
	for _, p := range pkgs {
		dir := relPath(root, p.Dir)
		t.importPaths[dir] = p.ImportPath
		if hasBenchmarks(p.Dir, append(p.TestGoFiles, p.XTestGoFiles...)) {
			t.bench[p.ImportPath] = benchPackage{moduleDir: moduleDir, dir: dir}
			benchPkgs = append(benchPkgs, p.ImportPath)
		}
	}
	if len(benchPkgs) == 0 {
		return nil
	}

	type testMain struct {
		ImportPath, Name string
		Deps             []string
	}
	// With -test, each package gets a generated "pkg.test" main package whose
	// Deps cover both its internal and external tests.
	mains, err := goList[testMain](ctx, modDir, append([]string{"-test", "-json=ImportPath,Name,Deps"}, benchPkgs...)...)
	if err != nil {
		return err
	}
	for _, m := range mains {
		pkg, ok := strings.CutSuffix(m.ImportPath, ".test")
		bp, isBench := t.bench[pkg]
		if !ok || m.Name != "main" || !isBench {
			continue
		}
		for _, dep := range m.Deps {
			// Test variants are listed as "pkg [pkg.test]".
			dep, _, _ = strings.Cut(dep, " ")
			bp.deps = append(bp.deps, dep)
		}
		t.bench[pkg] = bp
	}
	return nil
}

func hasBenchmarks(dir string, testFiles []string) bool {
	for _, f := range testFiles {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err == nil && benchmarkFunc.Match(data) {
			return true
		}
	}
	return false
}

// selectPackages returns the benchmark packages, from either checkout, that
// changed, depend on a changed package, or belong to a module whose go.mod or
// go.sum changed.
func selectPackages(changed []string, base, head tree, all bool) []Package {
	changedPkgs := map[string]bool{}
	changedMods := map[string]bool{}
	for _, file := range changed {
		dir := path.Dir(file)
		switch {
		case path.Base(file) == "go.mod" || path.Base(file) == "go.sum":
			changedMods[dir] = true
		case strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go"):
			for _, t := range []tree{base, head} {
				if pkg, ok := t.importPaths[dir]; ok {
					changedPkgs[pkg] = true
				}
			}
		}
	}

	affected := func(bp benchPackage) bool {
		if all || changedMods[bp.moduleDir] {
			return true
		}
		for _, file := range changed {
			if path.Dir(file) == bp.dir || strings.HasPrefix(file, bp.dir+"/testdata/") {
				return true
			}
		}
		for _, dep := range bp.deps {
			if changedPkgs[dep] {
				return true
			}
		}
		return false
	}

	selected := map[string]Package{}
	for _, t := range []tree{base, head} {
		for pkg, bp := range t.bench {
			if _, done := selected[pkg]; done || !affected(bp) {
				continue
			}
			_, inBase := base.bench[pkg]
			_, inHead := head.bench[pkg]
			selected[pkg] = Package{ImportPath: pkg, ModuleDir: bp.moduleDir, Dir: bp.dir, InBase: inBase, InHead: inHead}
		}
	}

	pkgs := make([]Package, 0, len(selected))
	for _, p := range selected {
		pkgs = append(pkgs, p)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })
	return pkgs
}

// goList runs go list -e in dir and decodes its JSON output.
func goList[T any](ctx context.Context, dir string, args ...string) ([]T, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", append([]string{"list", "-e", "-tags", buildTags}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list in %s: %w: %s", dir, err, stderr.String())
	}

	var out []T
	for dec := json.NewDecoder(&stdout); ; {
		var v T
		if err := dec.Decode(&v); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
}

func relPath(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}
