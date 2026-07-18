package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testArtifacts = `{
  "darwin": {
    "arm64": {"package": "alloy-darwin-arm64.zip", "binFile": "alloy-darwin-arm64"},
    "amd64": {"package": "alloy-darwin-amd64.zip", "binFile": "alloy-darwin-amd64"}
  },
  "linux": {
    "arm64": {"package": "alloy-linux-arm64.zip", "binFile": "alloy-linux-arm64"},
    "amd64": {"package": "alloy-linux-amd64.zip", "binFile": "alloy-linux-amd64"}
  }
}`

const testTemplate = `version "{{.Version}}"
tag {{.Tag}}
darwin-arm64 {{.BaseURL}}/{{.Tag}}/{{.Artifacts.Darwin.Arm64.Package}} {{.Artifacts.Darwin.Arm64.Checksum}} {{.Artifacts.Darwin.Arm64.BinFile}}
darwin-amd64 {{.Artifacts.Darwin.Amd64.Package}} {{.Artifacts.Darwin.Amd64.Checksum}}
linux-arm64 {{.Artifacts.Linux.Arm64.Package}} {{.Artifacts.Linux.Arm64.Checksum}}
linux-amd64 {{.Artifacts.Linux.Amd64.Package}} {{.Artifacts.Linux.Amd64.Checksum}}
`

// h is a valid-looking 64-char hex checksum builder for tests.
func h(c byte) string { return strings.Repeat(string(c), 64) }

func testSums() string {
	// Includes binary-mode marker (*) and extra unrelated line to exercise parsing.
	return h('a') + "  alloy-darwin-arm64.zip\n" +
		h('b') + " *alloy-darwin-amd64.zip\n" +
		h('c') + "  alloy-linux-arm64.zip\n" +
		h('d') + "  alloy-linux-amd64.zip\n" +
		h('e') + "  SHA256SUMS\n"
}

func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	artifactsPth := writeFile(t, dir, "artifacts.json", testArtifacts)
	templatePth := writeFile(t, dir, "alloy.rb.tpl", testTemplate)
	outPth := filepath.Join(dir, "alloy.rb")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/grafana/alloy/releases/download/v1.17.1/SHA256SUMS" {
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(testSums()))
	}))
	defer ts.Close()

	t.Setenv("GITHUB_SERVER_URL", ts.URL)
	t.Setenv("GITHUB_REPOSITORY", "grafana/alloy")

	err := run([]string{
		"-tag", "v1.17.1",
		"-artifacts", artifactsPth,
		"-template", templatePth,
		"-out", outPth,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	got, err := os.ReadFile(outPth)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}

	want := `version "1.17.1"
tag v1.17.1
darwin-arm64 ` + ts.URL + `/grafana/alloy/releases/download/v1.17.1/alloy-darwin-arm64.zip ` + h('a') + ` alloy-darwin-arm64
darwin-amd64 alloy-darwin-amd64.zip ` + h('b') + `
linux-arm64 alloy-linux-arm64.zip ` + h('c') + `
linux-amd64 alloy-linux-amd64.zip ` + h('d') + `
`
	if string(got) != want {
		t.Fatalf("rendered mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRunMissingChecksum(t *testing.T) {
	dir := t.TempDir()
	artifactsPth := writeFile(t, dir, "artifacts.json", testArtifacts)
	templatePth := writeFile(t, dir, "alloy.rb.tpl", testTemplate)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Omit linux-amd64 on purpose.
		_, _ = w.Write([]byte(h('a') + "  alloy-darwin-arm64.zip\n"))
	}))
	defer ts.Close()

	t.Setenv("GITHUB_SERVER_URL", ts.URL)
	t.Setenv("GITHUB_REPOSITORY", "grafana/alloy")

	err := run([]string{
		"-tag", "v1.17.1",
		"-artifacts", artifactsPth,
		"-template", templatePth,
		"-out", filepath.Join(dir, "alloy.rb"),
	})
	if err == nil || !strings.Contains(err.Error(), "missing checksums") {
		t.Fatalf("expected missing checksums error, got: %v", err)
	}
}

func TestRunRequiresEnv(t *testing.T) {
	dir := t.TempDir()
	artifactsPth := writeFile(t, dir, "artifacts.json", testArtifacts)

	t.Setenv("GITHUB_SERVER_URL", "")
	t.Setenv("GITHUB_REPOSITORY", "grafana/alloy")

	err := run([]string{"-tag", "v1.17.1", "-artifacts", artifactsPth})
	if err == nil || !strings.Contains(err.Error(), "GITHUB_SERVER_URL") {
		t.Fatalf("expected GITHUB_SERVER_URL error, got: %v", err)
	}
}

func TestParseChecksums(t *testing.T) {
	in := "aa  file-a\nbb *file-b\n\ncc\n" // blank line and single-field line are skipped
	got := parseChecksums(in)
	if got["file-a"] != "aa" || got["file-b"] != "bb" {
		t.Fatalf("unexpected parse: %#v", got)
	}
	if _, ok := got["cc"]; ok {
		t.Fatalf("single-field line should be skipped: %#v", got)
	}
}
