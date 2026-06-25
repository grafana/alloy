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
// Each archive has:
//
//	-- config.alloy --                  Alloy configuration to analyse (required)
//	-- expected_policy.yaml --          Expected generated policy YAML (required)
//	-- expected_dynamic_components --   Optional: one component name per line whose
//	                                    endpoints are unverifiable statically
//
// The test compares generated YAML and, if present, the DynamicComponents list.
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

			var configContent, expectedPolicy, expectedDynamic string
			for _, f := range archive.Files {
				switch f.Name {
				case "config.alloy":
					configContent = string(f.Data)
				case "expected_policy.yaml":
					expectedPolicy = string(f.Data)
				case "expected_dynamic_components":
					expectedDynamic = string(f.Data)
				}
			}
			require.NotEmpty(t, configContent, "txtar missing -- config.alloy --")
			require.NotEmpty(t, expectedPolicy, "txtar missing -- expected_policy.yaml --")

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

			if expectedDynamic != "" {
				var wantDynamic []string
				for _, line := range strings.Split(strings.TrimSpace(expectedDynamic), "\n") {
					if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
						wantDynamic = append(wantDynamic, line)
					}
				}
				require.ElementsMatch(t, wantDynamic, gp.DynamicComponents,
					"dynamic component list mismatch")
			}
		})
	}
}
