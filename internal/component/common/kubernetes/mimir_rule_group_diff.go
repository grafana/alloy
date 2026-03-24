package kubernetes

import (
	"bytes"

	"gopkg.in/yaml.v3" // Used for prometheus rulefmt compatibility instead of gopkg.in/yaml.v2

	"github.com/grafana/alloy/internal/mimir/client"
)

type MimirRuleGroupDiff struct {
	Kind    RuleGroupDiffKind
	Actual  client.MimirRuleGroup
	Desired client.MimirRuleGroup
}

type MimirRuleGroupsByNamespace map[string][]client.MimirRuleGroup
type MimirRuleGroupDiffsByNamespace map[string][]MimirRuleGroupDiff

func DiffMimirRuleGroupState(desired, actual MimirRuleGroupsByNamespace) MimirRuleGroupDiffsByNamespace {
	seenNamespaces := map[string]bool{}

	diff := make(MimirRuleGroupDiffsByNamespace)

	for namespace, desiredRuleGroups := range desired {
		seenNamespaces[namespace] = true

		actualRuleGroups := actual[namespace]
		subDiff := diffMimirRuleGroupNamespaceState(desiredRuleGroups, actualRuleGroups)

		if len(subDiff) == 0 {
			continue
		}

		diff[namespace] = subDiff
	}

	for namespace, actualRuleGroups := range actual {
		if seenNamespaces[namespace] {
			continue
		}

		subDiff := diffMimirRuleGroupNamespaceState(nil, actualRuleGroups)

		diff[namespace] = subDiff
	}

	return diff
}

func diffMimirRuleGroupNamespaceState(desired []client.MimirRuleGroup, actual []client.MimirRuleGroup) []MimirRuleGroupDiff {
	var diff []MimirRuleGroupDiff

	seenGroups := map[string]bool{}

desiredGroups:
	for _, desiredRuleGroup := range desired {
		seenGroups[desiredRuleGroup.Name] = true

		for _, actualRuleGroup := range actual {
			if desiredRuleGroup.Name == actualRuleGroup.Name {
				if equalMimirRuleGroups(desiredRuleGroup, actualRuleGroup) {
					continue desiredGroups
				}

				diff = append(diff, MimirRuleGroupDiff{
					Kind:    RuleGroupDiffKindUpdate,
					Actual:  actualRuleGroup,
					Desired: desiredRuleGroup,
				})
				continue desiredGroups
			}
		}

		diff = append(diff, MimirRuleGroupDiff{
			Kind:    RuleGroupDiffKindAdd,
			Desired: desiredRuleGroup,
		})
	}

	for _, actualRuleGroup := range actual {
		if seenGroups[actualRuleGroup.Name] {
			continue
		}

		diff = append(diff, MimirRuleGroupDiff{
			Kind:   RuleGroupDiffKindRemove,
			Actual: actualRuleGroup,
		})
	}

	return diff
}

func equalMimirRuleGroups(a, b client.MimirRuleGroup) bool {
	aBuf, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bBuf, err := yaml.Marshal(b)
	if err != nil {
		return false
	}

	return bytes.Equal(aBuf, bBuf)
}
