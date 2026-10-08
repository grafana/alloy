package benchdiff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var testModules = []module{
	{Dir: ".", Path: "github.com/grafana/alloy"},
	{Dir: "syntax", Path: "github.com/grafana/alloy/syntax"},
}

func TestOwningPackage(t *testing.T) {
	tests := []struct {
		file string
		want string
	}{
		{"internal/component/loki/process/stages/match.go", "github.com/grafana/alloy/internal/component/loki/process/stages"},
		{"main.go", "github.com/grafana/alloy"},
		{"syntax/vm/vm.go", "github.com/grafana/alloy/syntax/vm"},
		{"syntax/parser.go", "github.com/grafana/alloy/syntax"},
		{"syntaxfoo/x.go", "github.com/grafana/alloy/syntaxfoo"},
	}
	for _, tt := range tests {
		got, ok := owningPackage(tt.file, testModules)
		require.True(t, ok, tt.file)
		require.Equal(t, tt.want, got, tt.file)
	}
}

func TestSelectPackages(t *testing.T) {
	const (
		stages  = "github.com/grafana/alloy/internal/component/loki/process/stages"
		wal     = "github.com/grafana/alloy/internal/static/metrics/wal"
		vm      = "github.com/grafana/alloy/syntax/vm"
		removed = "github.com/grafana/alloy/internal/removed"
		added   = "github.com/grafana/alloy/internal/added"
		lokiPkg = "github.com/grafana/alloy/internal/component/common/loki"
	)

	base := tree{
		modules: testModules,
		bench: map[string]benchPackage{
			stages:  {ModuleDir: ".", Dir: "internal/component/loki/process/stages"},
			wal:     {ModuleDir: ".", Dir: "internal/static/metrics/wal"},
			vm:      {ModuleDir: "syntax", Dir: "syntax/vm"},
			removed: {ModuleDir: ".", Dir: "internal/removed"},
		},
		deps: map[string][]string{
			stages:  {lokiPkg, "github.com/grafana/alloy/syntax/scanner"},
			wal:     {"github.com/prometheus/prometheus/tsdb"},
			vm:      {"github.com/grafana/alloy/syntax/scanner"},
			removed: {"fmt"},
		},
	}
	head := tree{
		modules: testModules,
		bench: map[string]benchPackage{
			stages: base.bench[stages],
			wal:    base.bench[wal],
			vm:     base.bench[vm],
			added:  {ModuleDir: ".", Dir: "internal/added"},
		},
		deps: map[string][]string{
			stages: base.deps[stages],
			wal:    base.deps[wal],
			vm:     base.deps[vm],
			added:  {"fmt"},
		},
	}

	t.Run("reverse dependency", func(t *testing.T) {
		got := selectPackages([]string{"internal/component/common/loki/types.go"}, base, head, false)
		require.Equal(t, []Package{
			{ImportPath: stages, ModuleDir: ".", Dir: "internal/component/loki/process/stages", InBase: true, InHead: true, Reason: ReasonReverseDep},
		}, got)
	})

	t.Run("cross-module reverse dependency", func(t *testing.T) {
		got := selectPackages([]string{"syntax/scanner/scanner.go"}, base, head, false)
		require.Equal(t, []string{stages, vm}, importPaths(got))
		for _, p := range got {
			require.Equal(t, ReasonReverseDep, p.Reason)
		}
	})

	t.Run("test-only change elsewhere is ignored", func(t *testing.T) {
		got := selectPackages([]string{"internal/component/common/loki/types_test.go", "docs/README.md"}, base, head, false)
		require.Empty(t, got)
	})

	t.Run("direct change wins over reverse dependency", func(t *testing.T) {
		got := selectPackages([]string{
			"internal/component/loki/process/stages/match_test.go",
			"internal/component/common/loki/types.go",
		}, base, head, false)
		require.Len(t, got, 1)
		require.Equal(t, ReasonDirect, got[0].Reason)
	})

	t.Run("testdata change", func(t *testing.T) {
		got := selectPackages([]string{"internal/static/metrics/wal/testdata/segment"}, base, head, false)
		require.Equal(t, []string{wal}, importPaths(got))
		require.Equal(t, ReasonDirect, got[0].Reason)
	})

	t.Run("added and removed packages", func(t *testing.T) {
		got := selectPackages([]string{"internal/removed/bench_test.go", "internal/added/bench_test.go"}, base, head, false)
		require.Equal(t, []Package{
			{ImportPath: added, ModuleDir: ".", Dir: "internal/added", InBase: false, InHead: true, Reason: ReasonDirect},
			{ImportPath: removed, ModuleDir: ".", Dir: "internal/removed", InBase: true, InHead: false, Reason: ReasonDirect},
		}, got)
	})

	t.Run("go.mod change selects the whole module", func(t *testing.T) {
		got := selectPackages([]string{"syntax/go.mod"}, base, head, false)
		require.Equal(t, []string{vm}, importPaths(got))
		require.Equal(t, ReasonGoMod, got[0].Reason)
	})

	t.Run("all", func(t *testing.T) {
		got := selectPackages(nil, base, head, true)
		require.Equal(t, []string{added, stages, removed, wal, vm}, importPaths(got))
	})
}

func TestFindBenchDirs(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("go.mod", "module example.com/root\n")
	write("a/a_test.go", "package a\n\nfunc BenchmarkA(b *testing.B) {}\n")
	write("b/b_test.go", "package b\n\nfunc TestB(t *testing.T) {}\n")
	write("c/c.go", "package c\n\nfunc BenchmarkNotATest() {}\n")
	write("a/testdata/x/x_test.go", "package x\n\nfunc BenchmarkX(b *testing.B) {}\n")
	write("sub/go.mod", "module example.com/sub\n")
	write("sub/s/s_test.go", "package s\n\nfunc BenchmarkS(b *testing.B) {}\n")

	mods, err := findModules(root)
	require.NoError(t, err)
	require.Equal(t, []module{{Dir: ".", Path: "example.com/root"}, {Dir: "sub", Path: "example.com/sub"}}, mods)

	dirs, err := findBenchDirs(root, mods[0], mods)
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, dirs)

	dirs, err = findBenchDirs(root, mods[1], mods)
	require.NoError(t, err)
	require.Equal(t, []string{"sub/s"}, dirs)
	require.Equal(t, "example.com/sub/s", importPath(mods[1], "sub/s"))
	require.Equal(t, "example.com/sub", importPath(mods[1], "sub"))
}

func importPaths(pkgs []Package) []string {
	var paths []string
	for _, p := range pkgs {
		paths = append(paths, p.ImportPath)
	}
	return paths
}
