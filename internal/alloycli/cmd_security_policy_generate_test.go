package alloycli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

// TestSecurityPolicyGenerate runs every .txtar file in
// testdata/security_policy_generate/.
//
// Each archive has two sections:
//
//	-- config.alloy --          Alloy configuration to analyse
//	-- expected_policy.yaml --  Expected generated policy (YAML)
//
// The test compares the generated YAML against expected_policy.yaml
// after normalising whitespace.
func TestSecurityPolicyGenerate(t *testing.T) {
	const dir = "testdata/security_policy_generate"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txtar") {
			continue
		}
		e := e
		t.Run(strings.TrimSuffix(e.Name(), ".txtar"), func(t *testing.T) {
			archive, err := txtar.ParseFile(filepath.Join(dir, e.Name()))
			require.NoError(t, err)

			var configContent, expectedPolicy string
			for _, f := range archive.Files {
				switch f.Name {
				case "config.alloy":
					configContent = string(f.Data)
				case "expected_policy.yaml":
					expectedPolicy = string(f.Data)
				}
			}
			require.NotEmpty(t, configContent, "txtar missing -- config.alloy --")
			require.NotEmpty(t, expectedPolicy, "txtar missing -- expected_policy.yaml --")

			// Parse the config.
			source, err := alloy_runtime.ParseSource("config.alloy", []byte(configContent))
			require.NoError(t, err)

			gp := GeneratePolicy(source)
			gotYAML, err := policyYAML(gp)
			require.NoError(t, err)

			require.Equal(t,
				strings.TrimSpace(expectedPolicy),
				strings.TrimSpace(gotYAML),
				"generated policy mismatch",
			)
		})
	}
}
