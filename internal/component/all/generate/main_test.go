package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogMatchesSourceRegistrations(t *testing.T) {
	cat := loadTestCatalog(t)
	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	if err := auditCatalogSources(cat, repoRoot); err != nil {
		t.Fatalf("catalog does not match source registrations:\n%v", err)
	}
}

func TestScanRegistrationNamesResolvesConstantsAndPlatformDuplicates(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "component.go"), `package example

import alloycomponent "github.com/grafana/alloy/internal/component"

const namespace = "example"
const suffix = ".one"

var registration = alloycomponent.Registration{Name: namespace + suffix}

func init() {
	alloycomponent.Register(registration)
}
`)
	writeTestFile(t, filepath.Join(directory, "component_linux.go"), `//go:build linux

package example

import "github.com/grafana/alloy/internal/component"

func init() {
	component.Register(component.Registration{Name: "example.one"})
	component.Register(&component.Registration{Name: "example.two"})
}
`)

	scan, err := scanRegistrationNames(directory, "")
	if err != nil {
		t.Fatalf("scanRegistrationNames() returned error: %v", err)
	}
	want := []string{"example.one", "example.two"}
	if !reflect.DeepEqual(scan.Names, want) {
		t.Fatalf("registration names = %#v; want %#v", scan.Names, want)
	}
	if len(scan.Unresolved) != 0 {
		t.Fatalf("unresolved registrations = %#v; want none", scan.Unresolved)
	}
}

func TestRenderCatalogIsDeterministic(t *testing.T) {
	cat := loadTestCatalog(t)
	first, err := renderCatalog(cat)
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	second, err := renderCatalog(cat)
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("renderCatalog produced different output across calls")
	}
	if got, want := len(first), len(cat.Packages)+1; got != want {
		t.Fatalf("renderCatalog produced %d files; want %d", got, want)
	}

	full := string(first[fullImportFile])
	if !strings.Contains(full, "//go:build !alloy_custom_components") {
		t.Fatal("full import file is missing its default-build constraint")
	}
	if !strings.Contains(full, "// Import otelcol.exporter.awss3\n") {
		t.Fatal("full import file does not use the actual awss3 registration name")
	}
	if strings.Contains(full, "awss3exporter") {
		t.Fatal("full import file retains the stale awss3exporter comment")
	}

	aws := string(first["custom_discovery_aws.go"])
	wantConstraint := "//go:build alloy_custom_components && (alloy_component_discovery_aws || alloy_component_discovery_ec2 || alloy_component_discovery_lightsail)\n"
	if !strings.HasPrefix(aws, wantConstraint) {
		t.Fatalf("AWS selector constraint = %q; want prefix %q", firstLine(aws), strings.TrimSpace(wantConstraint))
	}
	if got := strings.Count(aws, `import _ "github.com/grafana/alloy/internal/component/discovery/aws"`); got != 1 {
		t.Fatalf("AWS selector contains %d blank imports; want 1", got)
	}

	remoteHTTP := string(first["custom_remote_http.go"])
	if got := strings.Count(remoteHTTP, `import _ "github.com/grafana/alloy/internal/component/remote/http"`); got != 1 {
		t.Fatalf("remote.http selector contains %d blank imports; want 1", got)
	}
	if strings.Contains(remoteHTTP, "Register(") {
		t.Fatal("remote.http selector must only blank-import the package")
	}
}

func TestGenerateCheckDetectsContentAndStaleFileDrift(t *testing.T) {
	cat := loadTestCatalog(t)
	files, err := renderCatalog(cat)
	if err != nil {
		t.Fatalf("render catalog: %v", err)
	}
	outDir := t.TempDir()
	if err := writeGenerated(outDir, files); err != nil {
		t.Fatalf("write generated files: %v", err)
	}
	if err := checkGenerated(outDir, files); err != nil {
		t.Fatalf("fresh generated files failed check: %v", err)
	}

	drifted := filepath.Join(outDir, "custom_discovery_aws.go")
	if err := os.WriteFile(drifted, []byte("package all\n"), 0o644); err != nil {
		t.Fatalf("write drifted file: %v", err)
	}
	if err := checkGenerated(outDir, files); err == nil || !strings.Contains(err.Error(), "content is stale") {
		t.Fatalf("content drift check error = %v; want content-is-stale error", err)
	}
	if err := writeGenerated(outDir, files); err != nil {
		t.Fatalf("rewrite generated files: %v", err)
	}

	stale := filepath.Join(outDir, "custom_removed_component.go")
	if err := os.WriteFile(stale, []byte("package all\n"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	if err := checkGenerated(outDir, files); err == nil || !strings.Contains(err.Error(), "stale generated files") {
		t.Fatalf("stale-file check error = %v; want stale-generated-files error", err)
	}
	if err := writeGenerated(outDir, files); err != nil {
		t.Fatalf("regenerate after stale file: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale generated file still exists; stat error: %v", err)
	}
}

func TestCheckedInGeneratedFilesAreCurrent(t *testing.T) {
	cat := loadTestCatalog(t)
	files, err := renderCatalog(cat)
	if err != nil {
		t.Fatalf("render catalog: %v", err)
	}
	if err := checkGenerated("..", files); err != nil {
		t.Fatalf("checked-in generated files are stale: %v", err)
	}
}

func loadTestCatalog(t *testing.T) catalog {
	t.Helper()
	return loadCatalog()
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return value[:index]
	}
	return value
}

func writeTestFile(t *testing.T, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}
