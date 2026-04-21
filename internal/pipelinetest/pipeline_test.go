package pipelinetest

import (
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

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bb, err := os.ReadFile(path)
			require.NoError(t, err)

			var schema TestSchema
			require.NoError(t, yaml.Unmarshal(bb, &schema))
			require.NoError(t, RunTest(schema, TestConfig{DataPath: t.TempDir()}))
		})
	}
}
