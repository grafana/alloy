package kubernetes

import (
	"bytes"

	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3" // Used for prometheus rulefmt compatibility instead of gopkg.in/yaml.v2
)

type RuleGroupDiffKind string

const (
	RuleGroupDiffKindAdd    RuleGroupDiffKind = "add"
	RuleGroupDiffKindRemove RuleGroupDiffKind = "remove"
	RuleGroupDiffKindUpdate RuleGroupDiffKind = "update"
)

type RuleGroupDiff struct {
	Kind    RuleGroupDiffKind
	Actual  rulefmt.RuleGroup
	Desired rulefmt.RuleGroup
}

type RuleGroupsByNamespace map[string][]rulefmt.RuleGroup
type RuleGroupDiffsByNamespace map[string][]RuleGroupDiff

func DiffRuleState(desired, actual RuleGroupsByNamespace) RuleGroupDiffsByNamespace {
	seenNamespaces := map[string]bool{}

	diff := make(RuleGroupDiffsByNamespace)

	for namespace, desiredRuleGroups := range desired {
		seenNamespaces[namespace] = true

		actualRuleGroups := actual[namespace]
		subDiff := diffRuleNamespaceState(desiredRuleGroups, actualRuleGroups)

		if len(subDiff) == 0 {
			continue
		}

		diff[namespace] = subDiff
	}

	for namespace, actualRuleGroups := range actual {
		if seenNamespaces[namespace] {
			continue
		}

		subDiff := diffRuleNamespaceState(nil, actualRuleGroups)

		diff[namespace] = subDiff
	}

	return diff
}

func diffRuleNamespaceState(desired []rulefmt.RuleGroup, actual []rulefmt.RuleGroup) []RuleGroupDiff {
	var diff []RuleGroupDiff

	seenGroups := map[string]bool{}

desiredGroups:
	for _, desiredRuleGroup := range desired {
		seenGroups[desiredRuleGroup.Name] = true

		for _, actualRuleGroup := range actual {
			if desiredRuleGroup.Name == actualRuleGroup.Name {
				if equalRuleGroups(desiredRuleGroup, actualRuleGroup) {
					continue desiredGroups
				}

				diff = append(diff, RuleGroupDiff{
					Kind:    RuleGroupDiffKindUpdate,
					Actual:  actualRuleGroup,
					Desired: desiredRuleGroup,
				})
				continue desiredGroups
			}
		}

		diff = append(diff, RuleGroupDiff{
			Kind:    RuleGroupDiffKindAdd,
			Desired: desiredRuleGroup,
		})
	}

	for _, actualRuleGroup := range actual {
		if seenGroups[actualRuleGroup.Name] {
			continue
		}

		diff = append(diff, RuleGroupDiff{
			Kind:   RuleGroupDiffKindRemove,
			Actual: actualRuleGroup,
		})
	}

	return diff
}

func equalRuleGroups(a, b rulefmt.RuleGroup) bool {
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
