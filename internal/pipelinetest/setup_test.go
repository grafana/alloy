package pipelinetest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunSetup_WritesFilesAndExpandsWorkDir(t *testing.T) {
	dir := t.TempDir()
	schema := TestSchema{
		Config: ConfigSchema(`pattern = "{{TEMP_DIR}}/probe.txt"`),
		Setup: SetupSchema{Files: []SetupFileSchema{
			{Path: "{{TEMP_DIR}}/nested/probe.txt", Content: "alloy"},
		}},
	}

	result, err := runSetup(schema, dir)
	require.NoError(t, err)
	defer result.cleanup()

	content, err := os.ReadFile(filepath.Join(dir, "nested", "probe.txt"))
	require.NoError(t, err)
	require.Equal(t, "alloy", string(content))
	require.Contains(t, result.config, "probe.txt")
	require.NotContains(t, result.config, tempDirVar)
}

func TestRunSetup_DoublesBackslashesInConfigOnly(t *testing.T) {
	result, err := runSetup(TestSchema{Config: ConfigSchema(`path = "{{TEMP_DIR}}"`)}, `C:\Users\test`)
	require.NoError(t, err)
	require.Equal(t, `path = "C:\\Users\\test"`, result.config)
}

// TestHelperProcess is not a real test. The setup tests run the test binary as
// a portable command, and this function is what the command does.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("PIPELINETEST_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	switch args[0] {
	case "fail":
		os.Exit(1)
	case "touch":
		if err := os.WriteFile(args[1], nil, 0o600); err != nil {
			os.Exit(2)
		}
	}
	os.Exit(0)
}

func helperCommand(args ...string) []string {
	return append([]string{os.Args[0], "-test.run=TestHelperProcess", "--"}, args...)
}

func TestRunSetup_RunsCleanupAfterFailedCommand(t *testing.T) {
	t.Setenv("PIPELINETEST_HELPER", "1")
	dir := t.TempDir()
	marker := filepath.Join(dir, "cleaned")
	schema := TestSchema{Setup: SetupSchema{Commands: []SetupCommandSchema{{
		Run:     helperCommand("fail"),
		Cleanup: helperCommand("touch", marker),
	}}}}

	result, err := runSetup(schema, dir)
	require.Error(t, err)
	require.NoFileExists(t, marker)

	result.cleanup()
	require.FileExists(t, marker)
}

func TestRunSetup_RejectsIncompleteSteps(t *testing.T) {
	_, err := runSetup(TestSchema{Setup: SetupSchema{Files: []SetupFileSchema{{}}}}, t.TempDir())
	require.Error(t, err)

	_, err = runSetup(TestSchema{Setup: SetupSchema{Commands: []SetupCommandSchema{{}}}}, t.TempDir())
	require.Error(t, err)
}
