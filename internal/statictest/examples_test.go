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

// TestRecipeRules loads every recipe's rules.yaml. When an example holds a
// recipe rule, it must be an exact copy, as the static test agent writes it.
func TestRecipeRules(t *testing.T) {
	examples := map[string]Rule{}
	tests, err := filepath.Glob("tests/*/*.statictest.yaml")
	require.NoError(t, err)
	for _, f := range tests {
		schema, err := LoadTestFile(f)
		require.NoError(t, err)
		for _, r := range schema.Rules {
			examples[r.Name] = r
		}
	}

	files, err := filepath.Glob("../../recipes/*/rules.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			rr, err := LoadRecipeRules(f)
			require.NoError(t, err)
			require.NotEmpty(t, rr.Rules)
			for _, r := range rr.Rules {
				if ex, ok := examples[r.Name]; ok && r.Static != nil {
					require.Equal(t, r.TestRule(), ex, r.Name)
				}
			}
		})
	}
}
