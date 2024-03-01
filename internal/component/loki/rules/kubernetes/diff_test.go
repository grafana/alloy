package rules

import (
	"fmt"
	"testing"

	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/stretchr/testify/require"
)

func parseRuleGroups(t *testing.T, buf []byte) []rulefmt.RuleGroup {
	t.Helper()

	groups, errs := rulefmt.Parse(buf)
	require.Empty(t, errs)

	return groups.Groups
}

func TestDiffRuleState(t *testing.T) {
	ruleGroupsA := parseRuleGroups(t, []byte(`
groups:
- name: rule-group-a
  interval: 1m
  rules:
  - record: rule_a
    expr: 1
`))

	ruleGroupsAModified := parseRuleGroups(t, []byte(`
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
		expected map[string][]ruleGroupDiff
	}

	testCases := []testCase{
		{
			name:     "empty sets",
			desired:  map[string][]rulefmt.RuleGroup{},
			actual:   map[string][]rulefmt.RuleGroup{},
			expected: map[string][]ruleGroupDiff{},
		},
		{
			name: "add rule group",
			desired: map[string][]rulefmt.RuleGroup{
				managedNamespace: ruleGroupsA,
			},
			actual: map[string][]rulefmt.RuleGroup{},
			expected: map[string][]ruleGroupDiff{
				managedNamespace: {
					{
						Kind:    ruleGroupDiffKindAdd,
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
			expected: map[string][]ruleGroupDiff{
				managedNamespace: {
					{
						Kind:   ruleGroupDiffKindRemove,
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
			expected: map[string][]ruleGroupDiff{
				managedNamespace: {
					{
						Kind:    ruleGroupDiffKindUpdate,
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
			expected: map[string][]ruleGroupDiff{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := diffRuleState(tc.desired, tc.actual)
			requireEqualRuleDiffs(t, tc.expected, actual)
		})
	}
}

func requireEqualRuleDiffs(t *testing.T, expected, actual map[string][]ruleGroupDiff) {
	require.Equal(t, len(expected), len(actual))

	var summarizeDiff = func(diff ruleGroupDiff) string {
		switch diff.Kind {
		case ruleGroupDiffKindAdd:
			return fmt.Sprintf("add: %s", diff.Desired.Name)
		case ruleGroupDiffKindRemove:
			return fmt.Sprintf("remove: %s", diff.Actual.Name)
		case ruleGroupDiffKindUpdate:
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
				!equalRuleGroups(expectedDiff.Desired, actualDiff.Desired) ||
				!equalRuleGroups(expectedDiff.Actual, actualDiff.Actual) {

				t.Logf("expected diff: %s", summarizeDiff(expectedDiff))
				t.Logf("actual diff: %s", summarizeDiff(actualDiff))
				t.Fail()
			}
		}
	}
}
