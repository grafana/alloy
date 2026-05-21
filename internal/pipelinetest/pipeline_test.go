package pipelinetest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPipelines(t *testing.T) {
	entries, err := os.ReadDir("tests")
	require.NoError(t, err)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		path := filepath.Join("tests", name, "test.yaml")

		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		require.NoError(t, err)

		t.Run(name, func(t *testing.T) {
			bb, err := os.ReadFile(path)
			require.NoError(t, err)

			var schema TestSchema
			require.NoError(t, yaml.Unmarshal(bb, &schema))
			require.NoError(t, RunTest(t.Context(), schema, TestConfig{DataPath: t.TempDir()}))
		})
	}
}
