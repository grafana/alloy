package benchdiff

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectPackages(t *testing.T) {
	const (
		stages  = "github.com/grafana/alloy/internal/component/loki/process/stages"
		wal     = "github.com/grafana/alloy/internal/static/metrics/wal"
		vm      = "github.com/grafana/alloy/syntax/vm"
		removed = "github.com/grafana/alloy/internal/removed"
		added   = "github.com/grafana/alloy/internal/added"
		loki    = "github.com/grafana/alloy/internal/component/common/loki"
		scanner = "github.com/grafana/alloy/syntax/scanner"
	)

	importPaths := map[string]string{
		"internal/component/loki/process/stages": stages,
		"internal/static/metrics/wal":            wal,
		"internal/component/common/loki":         loki,
		"syntax/vm":                              vm,
		"syntax/scanner":                         scanner,
		"internal/removed":                       removed,
		"internal/added":                         added,
	}
	stagesPkg := benchPackage{moduleDir: ".", dir: "internal/component/loki/process/stages", deps: []string{loki, scanner}}
	walPkg := benchPackage{moduleDir: ".", dir: "internal/static/metrics/wal", deps: []string{"github.com/prometheus/prometheus/tsdb"}}
	vmPkg := benchPackage{moduleDir: "syntax", dir: "syntax/vm", deps: []string{scanner}}

	base := tree{importPaths: importPaths, bench: map[string]benchPackage{
		stages: stagesPkg, wal: walPkg, vm: vmPkg,
		removed: {moduleDir: ".", dir: "internal/removed"},
	}}
	head := tree{importPaths: importPaths, bench: map[string]benchPackage{
		stages: stagesPkg, wal: walPkg, vm: vmPkg,
		added: {moduleDir: ".", dir: "internal/added"},
	}}

	tests := []struct {
		name    string
		changed []string
		all     bool
		want    []string
	}{
		{"reverse dependency", []string{"internal/component/common/loki/types.go"}, false, []string{stages}},
		{"reverse dependency across modules", []string{"syntax/scanner/scanner.go"}, false, []string{stages, vm}},
		{"test files and other files elsewhere", []string{"internal/component/common/loki/types_test.go", "docs/README.md"}, false, nil},
		{"test file in the package", []string{"internal/static/metrics/wal/wal_test.go"}, false, []string{wal}},
		{"testdata", []string{"internal/static/metrics/wal/testdata/segment"}, false, []string{wal}},
		{"go.mod", []string{"syntax/go.mod"}, false, []string{vm}},
		{"added and removed packages", []string{"internal/removed/bench_test.go", "internal/added/bench_test.go"}, false, []string{added, removed}},
		{"all", nil, true, []string{added, stages, removed, wal, vm}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, p := range selectPackages(tt.changed, base, head, tt.all) {
				got = append(got, p.ImportPath)
				require.Equal(t, p.ImportPath != added, p.InBase, p.ImportPath)
				require.Equal(t, p.ImportPath != removed, p.InHead, p.ImportPath)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoadTree(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("go.mod", "module example.com/root\n\ngo 1.26\n")
	write("a/a.go", "package a\n")
	write("a/a_test.go", "package a\n\nimport \"testing\"\n\nfunc BenchmarkA(b *testing.B) {}\n")
	write("b/b.go", "package b\n\nimport _ \"example.com/root/a\"\n")
	write("b/b_test.go", "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n")
	write("c/c_test.go", "package c_test\n\nimport (\n\t\"testing\"\n\n\t_ \"example.com/root/b\"\n)\n\nfunc BenchmarkC(b *testing.B) {}\n")
	write("sub/go.mod", "module example.com/sub\n\ngo 1.26\n")
	write("sub/s/s_test.go", "package s\n\nimport \"testing\"\n\nfunc BenchmarkS(b *testing.B) {}\n")

	tr, err := loadTree(context.Background(), root)
	require.NoError(t, err)

	require.Equal(t, "example.com/root/b", tr.importPaths["b"])
	require.Equal(t, "example.com/sub/s", tr.importPaths["sub/s"])
	require.ElementsMatch(t, []string{"example.com/root/a", "example.com/root/c", "example.com/sub/s"}, slices.Collect(maps.Keys(tr.bench)))
	require.Equal(t, "sub", tr.bench["example.com/sub/s"].moduleDir)
	require.Contains(t, tr.bench["example.com/root/c"].deps, "example.com/root/a", "transitive dependency of an external test")
}
