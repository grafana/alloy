package kubernetes

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/mimir/mimirclient"
)

func parseMimirRuleGroups(t *testing.T, buf []byte) []mimirclient.MimirRuleGroup {
	t.Helper()

	groups, errs := mimirclient.Parse(buf)
	require.Empty(t, errs)

	return groups.Groups
}

func TestDiffMimirRuleGroupState(t *testing.T) {
	ruleGroupsA := parseMimirRuleGroups(t, []byte(`
groups:
- name: rule-group-a
  interval: 1m
  source_tenants: ["tenant1","tenant2"]
  rules:
  - record: rule_a
    expr: 1
`))

	ruleGroupsAModified := parseMimirRuleGroups(t, []byte(`
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
		desired  map[string][]mimirclient.MimirRuleGroup
		actual   map[string][]mimirclient.MimirRuleGroup
		expected map[string][]MimirRuleGroupDiff
	}

	testCases := []testCase{
		{
			name:     "empty sets",
			desired:  map[string][]mimirclient.MimirRuleGroup{},
			actual:   map[string][]mimirclient.MimirRuleGroup{},
			expected: map[string][]MimirRuleGroupDiff{},
		},
		{
			name: "add rule group",
			desired: map[string][]mimirclient.MimirRuleGroup{
				managedNamespace: ruleGroupsA,
			},
			actual: map[string][]mimirclient.MimirRuleGroup{},
			expected: map[string][]MimirRuleGroupDiff{
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
			desired: map[string][]mimirclient.MimirRuleGroup{},
			actual: map[string][]mimirclient.MimirRuleGroup{
				managedNamespace: ruleGroupsA,
			},
			expected: map[string][]MimirRuleGroupDiff{
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
			desired: map[string][]mimirclient.MimirRuleGroup{
				managedNamespace: ruleGroupsA,
			},
			actual: map[string][]mimirclient.MimirRuleGroup{
				managedNamespace: ruleGroupsAModified,
			},
			expected: map[string][]MimirRuleGroupDiff{
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
			desired: map[string][]mimirclient.MimirRuleGroup{
				managedNamespace: ruleGroupsA,
			},
			actual: map[string][]mimirclient.MimirRuleGroup{
				managedNamespace: ruleGroupsA,
			},
			expected: map[string][]MimirRuleGroupDiff{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := DiffMimirRuleGroupState(tc.desired, tc.actual)
			requireEqualMimirRuleGroupDiffs(t, tc.expected, actual)
		})
	}
}

func requireEqualMimirRuleGroupDiffs(t *testing.T, expected, actual map[string][]MimirRuleGroupDiff) {
	require.Equal(t, len(expected), len(actual))

	var summarizeDiff = func(diff MimirRuleGroupDiff) string {
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
				!equalMimirRuleGroups(expectedDiff.Desired, actualDiff.Desired) ||
				!equalMimirRuleGroups(expectedDiff.Actual, actualDiff.Actual) {

				t.Logf("expected diff: %s", summarizeDiff(expectedDiff))
				t.Logf("actual diff: %s", summarizeDiff(actualDiff))
				t.Fail()
			}
		}
	}
}
