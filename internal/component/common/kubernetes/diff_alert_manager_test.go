package kubernetes

import (
	"fmt"
	"testing"

	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"
)

func TestDiffConfigs(t *testing.T) {
	cfg1 := alertmgr_cfg.Config{
		Receivers: []alertmgr_cfg.Receiver{
			{
				EmailConfigs: []*alertmgr_cfg.EmailConfig{
					{To: "James"},
				},
			},
		},
	}

	cfg2 := alertmgr_cfg.Config{
		Receivers: []alertmgr_cfg.Receiver{
			{
				EmailConfigs: []*alertmgr_cfg.EmailConfig{
					{To: "Dave"},
				},
			},
		},
	}

	managedNamespace := "agent/namespace/name/12345678-1234-1234-1234-123456789012"

	type testCase struct {
		name     string
		desired  map[string][]alertmgr_cfg.Config
		actual   map[string][]alertmgr_cfg.Config
		expected map[string][]AlertManagerConfigDiff
	}

	testCases := []testCase{
		// {
		// 	name:     "empty sets",
		// 	desired:  map[string][]alertmgr_cfg.Config{},
		// 	actual:   map[string][]alertmgr_cfg.Config{},
		// 	expected: map[string][]AlertManagerConfigDiff{},
		// },
		// {
		// 	name: "add config",
		// 	desired: map[string][]alertmgr_cfg.Config{
		// 		managedNamespace: []alertmgr_cfg.Config{cfg1},
		// 	},
		// 	actual: map[string][]alertmgr_cfg.Config{},
		// 	expected: map[string][]AlertManagerConfigDiff{
		// 		managedNamespace: {
		// 			{
		// 				Kind:    AlertManagerConfigDiffKindAdd,
		// 				Desired: cfg1,
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	name:    "remove config",
		// 	desired: map[string][]alertmgr_cfg.Config{},
		// 	actual: map[string][]alertmgr_cfg.Config{
		// 		managedNamespace: []alertmgr_cfg.Config{cfg1},
		// 	},
		// 	expected: map[string][]AlertManagerConfigDiff{
		// 		managedNamespace: {
		// 			{
		// 				Kind:   AlertManagerConfigDiffKindRemove,
		// 				Actual: cfg1,
		// 			},
		// 		},
		// 	},
		// },
		{
			name: "update config",
			desired: map[string][]alertmgr_cfg.Config{
				managedNamespace: []alertmgr_cfg.Config{cfg1},
			},
			actual: map[string][]alertmgr_cfg.Config{
				managedNamespace: []alertmgr_cfg.Config{cfg2},
			},
			expected: map[string][]AlertManagerConfigDiff{
				managedNamespace: {
					{
						Kind:    AlertManagerConfigDiffKindUpdate,
						Desired: cfg1,
						Actual:  cfg2,
					},
				},
			},
		},
		// {
		// 	name: "unchanged rule groups",
		// 	desired: map[string][]rulefmt.RuleGroup{
		// 		managedNamespace: ruleGroupsA,
		// 	},
		// 	actual: map[string][]rulefmt.RuleGroup{
		// 		managedNamespace: ruleGroupsA,
		// 	},
		// 	expected: map[string][]RuleGroupDiff{},
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := DiffAlertManagerConfigs(tc.desired, tc.actual)
			requireEqualAlertManagerConfigDiffs(t, tc.expected, actual)
		})
	}
}

func requireEqualAlertManagerConfigDiffs(t *testing.T, expected, actual map[string][]AlertManagerConfigDiff) {
	require.Equal(t, len(expected), len(actual))

	var summarizeDiff = func(diff AlertManagerConfigDiff) string {
		switch diff.Kind {
		case AlertManagerConfigDiffKindAdd:
			return fmt.Sprintf("add: %s", diff.Desired.String())
		case AlertManagerConfigDiffKindRemove:
			return fmt.Sprintf("remove: %s", diff.Actual.String())
		case AlertManagerConfigDiffKindUpdate:
			return fmt.Sprintf("update: %s", diff.Desired.String())
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
				!equalAlertManagerConfigs(expectedDiff.Desired, actualDiff.Desired) ||
				!equalAlertManagerConfigs(expectedDiff.Actual, actualDiff.Actual) {

				t.Logf("expected diff: %s", summarizeDiff(expectedDiff))
				t.Logf("actual diff: %s", summarizeDiff(actualDiff))
				t.Fail()
			}
		}
	}
}
