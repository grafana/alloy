package kubernetes

import (
	"fmt"
	"testing"

	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/stretchr/testify/require"
)

func parsePrometheusRuleGroups(t *testing.T, buf []byte) []rulefmt.RuleGroup {
	t.Helper()

	groups, errs := rulefmt.Parse(buf, true)
	require.Empty(t, errs)

	return groups.Groups
}

func TestDiffPrometheusRuleState(t *testing.T) {
	ruleGroupsA := parsePrometheusRuleGroups(t, []byte(`
groups:
- name: rule-group-a
  interval: 1m
  rules:
  - record: rule_a
    expr: 1
`))

	ruleGroupsAModified := parsePrometheusRuleGroups(t, []byte(`
groups:
- name: rule-group-a
  interval: 1m
  rules:
  - record: rule_a
    expr: 3
`))

	managedNamespace := "agent/namespace/name/12345678-1234-1234-1234-123456789012"

	type testCase struct {
		name     string
		desired  map[string][]rulefmt.RuleGroup
		actual   map[string][]rulefmt.RuleGroup
		expected map[string][]PrometheusRuleGroupDiff
	}

	testCases := []testCase{
		{
			name:     "empty sets",
			desired:  map[string][]rulefmt.RuleGroup{},
			actual:   map[string][]rulefmt.RuleGroup{},
			expected: map[string][]PrometheusRuleGroupDiff{},
		},
		{
			name: "add rule group",
			desired: map[string][]rulefmt.RuleGroup{
				managedNamespace: ruleGroupsA,
			},
			actual: map[string][]rulefmt.RuleGroup{},
			expected: map[string][]PrometheusRuleGroupDiff{
				managedNamespace: {
					{
						Kind:    RuleGroupDiffKindAdd,
						Desired: ruleGroupsA[0],
					},
				},
			},
		},
		{
			name:    "remove rule group",
			desired: map[string][]rulefmt.RuleGroup{},
			actual: map[string][]rulefmt.RuleGroup{
				managedNamespace: ruleGroupsA,
			},
			expected: map[string][]PrometheusRuleGroupDiff{
				managedNamespace: {
					{
						Kind:   RuleGroupDiffKindRemove,
						Actual: ruleGroupsA[0],
					},
				},
			},
		},
		{
			name: "update rule group",
			desired: map[string][]rulefmt.RuleGroup{
				managedNamespace: ruleGroupsA,
			},
			actual: map[string][]rulefmt.RuleGroup{
				managedNamespace: ruleGroupsAModified,
			},
			expected: map[string][]PrometheusRuleGroupDiff{
				managedNamespace: {
					{
						Kind:    RuleGroupDiffKindUpdate,
						Desired: ruleGroupsA[0],
						Actual:  ruleGroupsAModified[0],
					},
				},
			},
		},
		{
			name: "unchanged rule groups",
			desired: map[string][]rulefmt.RuleGroup{
				managedNamespace: ruleGroupsA,
			},
			actual: map[string][]rulefmt.RuleGroup{
				managedNamespace: ruleGroupsA,
			},
			expected: map[string][]PrometheusRuleGroupDiff{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := DiffPrometheusRuleGroupState(tc.desired, tc.actual)
			requireEqualPrometheusRuleGroupDiffs(t, tc.expected, actual)
		})
	}
}

func requireEqualPrometheusRuleGroupDiffs(t *testing.T, expected, actual map[string][]PrometheusRuleGroupDiff) {
	require.Equal(t, len(expected), len(actual))

	var summarizeDiff = func(diff PrometheusRuleGroupDiff) string {
		switch diff.Kind {
		case RuleGroupDiffKindAdd:
			return fmt.Sprintf("add: %s", diff.Desired.Name)
		case RuleGroupDiffKindRemove:
			return fmt.Sprintf("remove: %s", diff.Actual.Name)
		case RuleGroupDiffKindUpdate:
			return fmt.Sprintf("update: %s", diff.Desired.Name)
		}
		panic("unreachable")
	}

	for namespace, expectedDiffs := range expected {
		actualDiffs, ok := actual[namespace]
		require.True(t, ok)

		require.Equal(t, len(expectedDiffs), len(actualDiffs))

		for i, expectedDiff := range expectedDiffs {
			actualDiff := actualDiffs[i]

			if expectedDiff.Kind != actualDiff.Kind ||
				!equalPrometheusRuleGroups(expectedDiff.Desired, actualDiff.Desired) ||
				!equalPrometheusRuleGroups(expectedDiff.Actual, actualDiff.Actual) {

				t.Logf("expected diff: %s", summarizeDiff(expectedDiff))
				t.Logf("actual diff: %s", summarizeDiff(actualDiff))
				t.Fail()
			}
		}
	}
}
