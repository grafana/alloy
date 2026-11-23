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

	config, err := runSetup(schema, dir)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "nested", "probe.txt"))
	require.NoError(t, err)
	require.Equal(t, "alloy", string(content))
	require.Contains(t, config, "probe.txt")
	require.NotContains(t, config, tempDirVar)
}

func TestRunSetup_DoublesBackslashesInConfigOnly(t *testing.T) {
	config, err := runSetup(TestSchema{Config: ConfigSchema(`path = "{{TEMP_DIR}}"`)}, `C:\Users\test`)
	require.NoError(t, err)
	require.Equal(t, `path = "C:\\Users\\test"`, config)
}

func TestRunSetup_RejectsPlaceholderWithoutWorkDir(t *testing.T) {
	_, err := runSetup(TestSchema{Config: ConfigSchema(`path = "{{TEMP_DIR}}"`)}, "")
	require.Error(t, err)

	_, err = runSetup(TestSchema{Setup: SetupSchema{Files: []SetupFileSchema{{Path: "{{TEMP_DIR}}/a"}}}}, "")
	require.Error(t, err)

	// A test that does not use the placeholder needs no working directory.
	_, err = runSetup(TestSchema{Config: ConfigSchema(`a = 1`)}, "")
	require.NoError(t, err)
}

func TestRunSetup_RejectsFileWithoutPath(t *testing.T) {
	_, err := runSetup(TestSchema{Setup: SetupSchema{Files: []SetupFileSchema{{}}}}, t.TempDir())
	require.Error(t, err)
}
