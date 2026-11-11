package pipelinetest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPipelines(t *testing.T) {
	found, err := FindTestFiles([]string{"tests"}, DefaultMatch(Kind), []string{Kind})
	require.NoError(t, err)
	require.Empty(t, found.Skipped, "every matched file must have kind: pipelinetest")
	require.NotEmpty(t, found.Files)

	for _, path := range found.Files {
		name := strings.TrimSuffix(filepath.Base(path), ".pipelinetest.yaml")
		t.Run(name, func(t *testing.T) {
			result, err := RunTestFile(t.Context(), path, TestConfig{DataPath: t.TempDir()})
			require.NoError(t, err)
			require.NoError(t, result.Err())
		})
	}
}
