package statictest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExamples runs every static test under tests/. Each one must pass.
func TestExamples(t *testing.T) {
	files, err := filepath.Glob("tests/*/*.statictest.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			res, err := RunFile(f)
			require.NoError(t, err)
			for _, rr := range res.Rules {
				require.True(t, rr.Passed, "%s: %s", rr.Rule.Name, rr.Message)
			}
		})
	}
}
