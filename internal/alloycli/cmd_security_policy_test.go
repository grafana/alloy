package alloycli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/securitypolicy"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

// TestSecurityPolicyCheck runs every .txtar file in testdata/security_policy_check/.
//
// Each archive contains three sections:
//
//	-- config.alloy --   Alloy configuration to check
//	-- policy.yaml --    Security policy to apply
//	-- result --         Expected findings (described below)
//
// Result format (one directive per line; blank lines and # comments ignored):
//
//	violations: N                            — total expected violation count
//	dynamic: N                               — total expected unverifiable endpoint count
//	component_violation: <name> <label>      — a component that should be blocked
//	endpoint_violation: <name> <label> <url> — an endpoint that should be blocked
//	endpoint_allowed: <name> <label> <url>   — an endpoint that should pass
//	dynamic_endpoint: <name> <label>         — an endpoint flagged as unverifiable
func TestSecurityPolicyCheck(t *testing.T) {
	const dir = "testdata/security_policy_check"
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

			tc := parseTxtar(t, archive)

			// Write config and policy to temp files so CheckConfig can read them.
			tmp := t.TempDir()
			configPath := filepath.Join(tmp, "config.alloy")
			policyPath := filepath.Join(tmp, "policy.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tc.config), 0o600))
			require.NoError(t, os.WriteFile(policyPath, []byte(tc.policy), 0o600))

			policy, err := securitypolicy.LoadFromFile(policyPath)
			require.NoError(t, err)

			report, err := CheckConfig(policy, configPath, "alloy")
			require.NoError(t, err)

			assertReport(t, report, tc.expected)
		})
	}
}

type txtarTestCase struct {
	config   string
	policy   string
	expected expectedResult
}

func parseTxtar(t *testing.T, archive *txtar.Archive) txtarTestCase {
	t.Helper()
	var tc txtarTestCase
	for _, f := range archive.Files {
		switch f.Name {
		case "config.alloy":
			tc.config = string(f.Data)
		case "policy.yaml":
			tc.policy = string(f.Data)
		case "result":
			tc.expected = parseResultLines(t, string(f.Data))
		}
	}
	require.NotEmpty(t, tc.config, "txtar missing -- config.alloy --")
	require.NotEmpty(t, tc.policy, "txtar missing -- policy.yaml --")
	return tc
}

// expectedResult holds the parsed expectations from a result section.
type expectedResult struct {
	violations          int
	dynamic             int
	componentViolations []string // "<name> <label>"
	endpointViolations  []string // "<name> <label> <url>"
	endpointAllowed     []string // "<name> <label> <url>"
	dynamicEndpoints    []string // "<name> <label>"
}

func parseResultLines(t *testing.T, content string) expectedResult {
	t.Helper()
	var out expectedResult
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, _ := strings.Cut(line, ": ")
		switch key {
		case "violations":
			fmt.Sscanf(val, "%d", &out.violations)
		case "dynamic":
			fmt.Sscanf(val, "%d", &out.dynamic)
		case "component_violation":
			out.componentViolations = append(out.componentViolations, val)
		case "endpoint_violation":
			out.endpointViolations = append(out.endpointViolations, val)
		case "endpoint_allowed":
			out.endpointAllowed = append(out.endpointAllowed, val)
		case "dynamic_endpoint":
			out.dynamicEndpoints = append(out.dynamicEndpoints, val)
		}
	}
	require.NoError(t, scanner.Err())
	return out
}

func assertReport(t *testing.T, report *PolicyCheckReport, exp expectedResult) {
	t.Helper()
	require.Equal(t, exp.violations, report.Violations, "violation count mismatch")
	require.Equal(t, exp.dynamic, report.Dynamic, "dynamic endpoint count mismatch")

	blockedComponents := map[string]bool{}
	blockedEndpoints := map[string]bool{}
	allowedEndpoints := map[string]bool{}
	dynamicEndpoints := map[string]bool{}

	for _, c := range report.Components {
		compKey := fmt.Sprintf("%s %s", c.Name, c.Label)
		if c.ComponentViolation != "" {
			blockedComponents[compKey] = true
		}
		for _, ep := range c.EndpointFindings {
			epKey := fmt.Sprintf("%s %s %s", c.Name, c.Label, ep.URL)
			switch {
			case ep.Dynamic:
				dynamicEndpoints[compKey] = true
			case ep.Violation != "":
				blockedEndpoints[epKey] = true
			default:
				allowedEndpoints[epKey] = true
			}
		}
	}

	for _, cv := range exp.componentViolations {
		require.True(t, blockedComponents[cv], "expected component violation %q not found in report", cv)
	}
	for _, ev := range exp.endpointViolations {
		require.True(t, blockedEndpoints[ev], "expected endpoint violation %q not found in report", ev)
	}
	for _, ea := range exp.endpointAllowed {
		require.True(t, allowedEndpoints[ea], "expected allowed endpoint %q not found in report", ea)
	}
	for _, de := range exp.dynamicEndpoints {
		require.True(t, dynamicEndpoints[de], "expected dynamic endpoint for %q not found in report", de)
	}
}
