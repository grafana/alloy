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
		cmd := pipelineTestCommand()
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
	require.Contains(t, out, "SKIP  "+filepath.Join(dir, "values.yaml")+" (no kind: pipelinetest)")
	require.Contains(t, out, "1 test: 1 passed, 1 skipped")
}
