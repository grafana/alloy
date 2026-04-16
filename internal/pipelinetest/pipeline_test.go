package pipelinetest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPipelines(t *testing.T) {
	entries, err := os.ReadDir("tests")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		path := filepath.Join("tests", entry.Name())

		t.Run(name, func(t *testing.T) {
			bb, err := os.ReadFile(path)
			require.NoError(t, err)

			var schema TestSchema
			require.NoError(t, yaml.Unmarshal(bb, &schema))
			require.NoError(t, RunTest(schema))
		})
	}
}
