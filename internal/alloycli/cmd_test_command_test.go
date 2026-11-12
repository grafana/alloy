package alloycli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPipelineTestCommand(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("config.alloy", `
loki.write "default" {
  endpoint {
    url = "http://example.com/loki/api/v1/push"
  }
}
`)
	test := func(line string) string {
		return `
kind: pipelinetest
version: 1
config:
  path: config.alloy
  sinks: [loki.write.default]
inputs:
  loki:
    - components: [loki.write.default.receiver]
      entries:
        - line: hello
assert:
  loki:
    - match: {line: ` + line + `}
`
	}
	write("ok.pipelinetest.yaml", test("hello"))

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := testCommand()
		cmd.SetArgs(args)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.Execute()
		return out.String(), err
	}

	out, err := run("--output", "json", dir)
	require.NoError(t, err)
	var report struct {
		Summary struct{ Total, Passed int }
	}
	require.NoError(t, json.Unmarshal([]byte(out), &report), out)
	require.Equal(t, 1, report.Summary.Total)
	require.Equal(t, 1, report.Summary.Passed)

	write("bad.pipelinetest.yaml", test("bye"))
	out, err = run("--color", "never", "--timeout", "500ms", dir)
	require.ErrorIs(t, err, errTestsFailed)
	require.Contains(t, out, "FAIL  "+filepath.Join(dir, "bad.pipelinetest.yaml"))
	require.Contains(t, out, "PASS  "+filepath.Join(dir, "ok.pipelinetest.yaml"))

	_, err = run("--output", "xml", dir)
	require.ErrorContains(t, err, "--output must be text or json")

	_, err = run(t.TempDir())
	require.ErrorContains(t, err, "no test files found")

	_, err = run("--match", "*.yaml", filepath.Join(dir, "ok.pipelinetest.yaml"))
	require.ErrorContains(t, err, "--match only applies to directories")

	write("values.yaml", "replicas: 3\n")
	out, err = run("--color", "never", "--match", "ok.*,values.yaml", dir)
	require.NoError(t, err)
	require.Contains(t, out, "SKIP  "+filepath.Join(dir, "values.yaml")+" (no kind: pipelinetest or statictest)")
	require.Contains(t, out, "1 test: 1 passed, 1 skipped")
}

func TestStaticTestCommand(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return path
	}
	write("config.alloy", `
prometheus.scrape "a" {
  targets    = []
  forward_to = []
  clustering {
    enabled = true
  }
}

prometheus.scrape "b" {
  targets    = []
  forward_to = []
}
`)
	static := write("config.statictest.yaml", `
kind: statictest
version: 1
config:
  path: config.alloy
rules:
  - name: clustering on all scrape components
    description: Without it, each instance scrapes every target.
    docs: https://grafana.com/docs/alloy/latest/get-started/clustering/
    type: all-or-none
    select: prometheus.scrape
    path: clustering.enabled
    equals: true
  - name: one scrape component
    severity: should
    type: count
    select: prometheus.scrape
    max: 1
`)
	pipeline := write("ok.pipelinetest.yaml", `
kind: pipelinetest
version: 1
config: |
  loki.process "p" {
    forward_to = [pipelinetest.loki.receiver]
  }
inputs:
  loki:
    - components: [loki.process.p.receiver]
      entries:
        - line: hello
assert:
  loki:
    - match: {line: hello}
`)

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := testCommand()
		cmd.SetArgs(args)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.Execute()
		return out.String(), err
	}

	out, err := run("static", "--color", "never", dir)
	require.ErrorIs(t, err, errTestsFailed)
	require.Contains(t, out, "FAIL  "+static)
	require.Contains(t, out, "✗ clustering on all scrape components")
	require.Contains(t, out, "doesn't: prometheus.scrape.b (config.alloy:10): clustering.enabled is not set")
	require.Contains(t, out, "why: Without it, each instance scrapes every target.")
	require.Contains(t, out, "! one scrape component (should)")
	require.NotContains(t, out, pipeline, "static runs only static tests")

	// With no subcommand, both kinds run.
	out, err = run("--color", "never", dir)
	require.ErrorIs(t, err, errTestsFailed)
	require.Contains(t, out, "PASS  "+pipeline)
	require.Contains(t, out, "FAIL  "+static)
	require.Contains(t, out, "2 tests: 1 passed, 1 failed")

	out, err = run("pipeline", "--color", "never", static)
	require.ErrorIs(t, err, errTestsFailed)
	require.Contains(t, out, "kind is statictest, but this command runs pipelinetest")

	out, err = run("static", "--output", "json", dir)
	require.ErrorIs(t, err, errTestsFailed)
	var report struct {
		Tests []struct {
			Kind       string
			Assertions []struct {
				Name, Severity, Description, Docs string
				Passed                            bool
			}
		}
	}
	require.NoError(t, json.Unmarshal([]byte(out), &report), out)
	require.Equal(t, "statictest", report.Tests[0].Kind)
	require.Equal(t, "https://grafana.com/docs/alloy/latest/get-started/clustering/", report.Tests[0].Assertions[0].Docs)
	require.Equal(t, "should", report.Tests[0].Assertions[1].Severity)
}
