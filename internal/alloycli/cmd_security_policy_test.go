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
)

// TestSecurityPolicyCheck runs every <name>.alloy + <name>.policy.yaml pair in
// testdata/security_policy_check/ and compares findings against <name>.result.
//
// Result file format (one directive per line, blank lines and # comments ignored):
//
//	violations: N                         — expected total violation count
//	dynamic: N                            — expected dynamic (unverifiable) endpoint count
//	component_violation: <name> <label>   — a component that should be blocked
//	endpoint_violation: <name> <label> <url> — an endpoint that should be blocked
//	endpoint_allowed: <name> <label> <url>   — an endpoint that should pass
//	dynamic_endpoint: <name> <label>         — an endpoint flagged as unverifiable
func TestSecurityPolicyCheck(t *testing.T) {
	dir := "testdata/security_policy_check"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".alloy") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".alloy")
		seen[base] = true
	}

	for base := range seen {
		base := base
		t.Run(base, func(t *testing.T) {
			configPath := filepath.Join(dir, base+".alloy")
			policyPath := filepath.Join(dir, base+".policy.yaml")
			resultPath := filepath.Join(dir, base+".result")

			policy, err := securitypolicy.LoadFromFile(policyPath)
			require.NoError(t, err, "loading policy %s", policyPath)

			report, err := CheckConfig(policy, configPath, "alloy")
			require.NoError(t, err, "CheckConfig failed for %s", configPath)

			expected := parseResultFile(t, resultPath)
			assertReport(t, report, expected)
		})
	}
}

// expectedResult holds the parsed expectations from a .result file.
type expectedResult struct {
	violations          int
	dynamic             int
	componentViolations []string // "<name> <label>"
	endpointViolations  []string // "<name> <label> <url>"
	endpointAllowed     []string // "<name> <label> <url>"
	dynamicEndpoints    []string // "<name> <label>"
}

func parseResultFile(t *testing.T, path string) expectedResult {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err, "opening result file %s", path)
	defer f.Close()

	var out expectedResult
	scanner := bufio.NewScanner(f)
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

	// Build lookup maps from the report.
	blockedComponents := map[string]bool{}
	blockedEndpoints := map[string]bool{}
	allowedEndpoints := map[string]bool{}
	dynamicEndpoints := map[string]bool{}

	for _, c := range report.Components {
		key := fmt.Sprintf("%s %s", c.Name, c.Label)
		if c.ComponentViolation != "" {
			blockedComponents[key] = true
		}
		for _, ep := range c.EndpointFindings {
			epKey := fmt.Sprintf("%s %s %s", c.Name, c.Label, ep.URL)
			switch {
			case ep.Dynamic:
				dynamicEndpoints[key] = true
			case ep.Violation != "":
				blockedEndpoints[epKey] = true
			default:
				allowedEndpoints[epKey] = true
			}
		}
	}

	for _, cv := range exp.componentViolations {
		require.True(t, blockedComponents[cv], "expected component violation %q not found", cv)
	}
	for _, ev := range exp.endpointViolations {
		require.True(t, blockedEndpoints[ev], "expected endpoint violation %q not found", ev)
	}
	for _, ea := range exp.endpointAllowed {
		require.True(t, allowedEndpoints[ea], "expected allowed endpoint %q not found", ea)
	}
	for _, de := range exp.dynamicEndpoints {
		require.True(t, dynamicEndpoints[de], "expected dynamic endpoint for %q not found", de)
	}
}
