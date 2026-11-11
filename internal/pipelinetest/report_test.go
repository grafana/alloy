package pipelinetest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const reportConfig = `
loki.process "default" {
  stage.static_labels {
    values = {env = "test"}
  }
  forward_to = [loki.write.default.receiver]
}

loki.write "default" {
  endpoint {
    url = "http://example.com/loki/api/v1/push"
  }
}
`

func TestRunFilesReport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.alloy"), reportConfig)

	testFile := func(configPath, want string) string {
		return `
kind: pipelinetest
version: 1
config:
  path: ` + configPath + `
  sinks: [loki.write.default]
inputs:
  loki:
    - components: [loki.process.default.receiver]
      entries:
        - line: hello
assert:
  loki:
    - name: env label added
      match:
        selector: '{env="` + want + `"}'
`
	}
	writeFile(t, filepath.Join(dir, "pass.pipelinetest.yaml"), testFile("config.alloy", "test"))
	writeFile(t, filepath.Join(dir, "sub", "fail.pipelinetest.yaml"), testFile("../config.alloy", "prod"))
	writeFile(t, filepath.Join(dir, "sub", "z.pipelinetest.yaml"), "kind: pipelinetest\nversion: 1\nconfig: 'not alloy {'\nassert:\n  loki:\n    - count: 1\n")
	writeFile(t, filepath.Join(dir, "ignored.yaml"), "not a test")

	files, err := FindTestFiles([]string{dir}, DefaultMatch(Kind), []string{Kind})
	require.NoError(t, err)
	require.Empty(t, files.Skipped)
	require.Equal(t, []string{
		filepath.Join(dir, "pass.pipelinetest.yaml"),
		filepath.Join(dir, "sub", "fail.pipelinetest.yaml"),
		filepath.Join(dir, "sub", "z.pipelinetest.yaml"),
	}, files.Files)

	var streamed []string
	report := RunFiles(t.Context(), files, RunOptions{
		AssertTimeout: time.Second,
		OnResult:      func(fr FileReport) { streamed = append(streamed, string(fr.Status)) },
	})

	require.Equal(t, []string{"passed", "failed", "error"}, streamed)
	require.Equal(t, Summary{Total: 3, Passed: 1, Failed: 1, Errors: 1, Skipped: 0, DurationSeconds: report.Summary.DurationSeconds}, report.Summary)
	require.False(t, report.Passed())

	failed := report.Tests[1]
	require.Equal(t, "loki[0] contains: env label added", failed.Assertions[0].Name)
	require.Contains(t, failed.Assertions[0].Message, `no matching entry found for selector {env="prod"}`)
	require.NotNil(t, failed.Snapshot)
	require.Equal(t, map[string]string{"env": "test"}, failed.Snapshot.Logs[0].Labels)

	require.Nil(t, report.Tests[0].Snapshot, "passing tests carry no snapshot")
	require.Contains(t, report.Tests[2].Error, "config:1")

	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, report))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	require.Contains(t, decoded, "summary")
	require.Equal(t, float64(ReportVersion), decoded["version"])

	var text bytes.Buffer
	r := TextReporter{W: &text}
	for _, fr := range report.Tests {
		r.File(fr)
	}
	r.Summary(report)
	require.Contains(t, text.String(), "✗ loki[0] contains: env label added")
	require.Contains(t, text.String(), "3 tests: 1 passed, 1 failed, 1 errors")
}

func TestFindTestFiles(t *testing.T) {
	dir := t.TempDir()
	test := "kind: pipelinetest\nversion: 1\n"
	writeFile(t, filepath.Join(dir, "a.pipelinetest.yaml"), test)
	writeFile(t, filepath.Join(dir, "b.pipelinetest.yml"), test)
	writeFile(t, filepath.Join(dir, "sub", "c.yaml"), test)
	writeFile(t, filepath.Join(dir, "sub", "values.yaml"), "replicas: 3\n")
	writeFile(t, filepath.Join(dir, "other", "d.yaml"), test)
	writeFile(t, filepath.Join(dir, ".github", "e.pipelinetest.yaml"), test)
	writeFile(t, filepath.Join(dir, "no-kind.pipelinetest.yaml"), "config: x\n")
	writeFile(t, filepath.Join(dir, "new.pipelinetest.yaml"), "kind: pipelinetest\nversion: 9\n")
	join := func(p ...string) string { return filepath.Join(append([]string{dir}, p...)...) }

	t.Run("default patterns", func(t *testing.T) {
		found, err := FindTestFiles([]string{dir}, DefaultMatch(Kind), []string{Kind})
		require.NoError(t, err)
		// A bad version is still a test, so the run shows its error. Hidden
		// directories are skipped.
		require.Equal(t, []string{join("a.pipelinetest.yaml"), join("b.pipelinetest.yml"), join("new.pipelinetest.yaml")}, found.Files)
		require.Equal(t, []SkippedFile{{File: join("no-kind.pipelinetest.yaml"), Reason: "no kind: pipelinetest"}}, found.Skipped)
	})

	t.Run("file name pattern", func(t *testing.T) {
		found, err := FindTestFiles([]string{dir}, []string{"*.yaml"}, []string{Kind})
		require.NoError(t, err)
		require.Equal(t, []string{join("a.pipelinetest.yaml"), join("new.pipelinetest.yaml"), join("other", "d.yaml"), join("sub", "c.yaml")}, found.Files)
		require.Equal(t, []SkippedFile{
			{File: join("no-kind.pipelinetest.yaml"), Reason: "no kind: pipelinetest"},
			{File: join("sub", "values.yaml"), Reason: "no kind: pipelinetest"},
		}, found.Skipped)
	})

	t.Run("path pattern", func(t *testing.T) {
		found, err := FindTestFiles([]string{dir}, []string{"sub/**/*.yaml"}, []string{Kind})
		require.NoError(t, err)
		require.Equal(t, []string{join("sub", "c.yaml")}, found.Files)
	})

	t.Run("a file always runs", func(t *testing.T) {
		found, err := FindTestFiles([]string{join("sub", "values.yaml")}, DefaultMatch(Kind), []string{Kind})
		require.NoError(t, err)
		require.Equal(t, []string{join("sub", "values.yaml")}, found.Files)
	})

	t.Run("invalid pattern", func(t *testing.T) {
		_, err := FindTestFiles([]string{dir}, []string{"[a"}, []string{Kind})
		require.ErrorContains(t, err, `invalid --match pattern "[a"`)
	})
}

// TestRunTestFilePaths checks that paths in a config are relative to the test
// file, and that a failure shows the error Alloy logged.
func TestRunTestFilePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sub", "app.log"), "hello\n")
	test := func(path string) string {
		return `
kind: pipelinetest
version: 1
config: |
  loki.source.file "f" {
    targets    = [{__path__ = "` + path + `"}]
    forward_to = [pipelinetest.loki.receiver]
  }
assert:
  loki:
    - match: {line: hello}
`
	}
	writeFile(t, filepath.Join(dir, "sub", "ok.pipelinetest.yaml"), test("./app.log"))
	writeFile(t, filepath.Join(dir, "sub", "missing.pipelinetest.yaml"), test("./missing.log"))

	wd, err := os.Getwd()
	require.NoError(t, err)

	result, err := RunTestFile(t.Context(), filepath.Join(dir, "sub", "ok.pipelinetest.yaml"), TestConfig{DataPath: t.TempDir()})
	require.NoError(t, err)
	require.NoError(t, result.Err())

	result, err = RunTestFile(t.Context(), filepath.Join(dir, "sub", "missing.pipelinetest.yaml"), TestConfig{
		DataPath:      t.TempDir(),
		AssertTimeout: time.Second,
	})
	require.NoError(t, err)
	require.False(t, result.Passed())
	require.NotEmpty(t, result.Logs)
	require.Contains(t, strings.Join(result.Logs, "\n"), "missing.log: no such file or directory")

	after, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, wd, after, "the working directory is restored")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
