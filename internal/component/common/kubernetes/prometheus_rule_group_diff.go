package kubernetes

import (
	"bytes"

	"gopkg.in/yaml.v3" // Used for prometheus rulefmt compatibility instead of gopkg.in/yaml.v2

	"github.com/prometheus/prometheus/model/rulefmt"
)

type PrometheusRuleGroupDiff struct {
	Kind    RuleGroupDiffKind
	Actual  rulefmt.RuleGroup
	Desired rulefmt.RuleGroup
}

type PrometheusRuleGroupsByNamespace map[string][]rulefmt.RuleGroup
type PrometheusRuleGroupDiffsByNamespace map[string][]PrometheusRuleGroupDiff

func DiffPrometheusRuleGroupState(desired, actual PrometheusRuleGroupsByNamespace) PrometheusRuleGroupDiffsByNamespace {
	seenNamespaces := map[string]bool{}

	diff := make(PrometheusRuleGroupDiffsByNamespace)

	for namespace, desiredRuleGroups := range desired {
		seenNamespaces[namespace] = true

		actualRuleGroups := actual[namespace]
		subDiff := diffPrometheusRuleNamespaceState(desiredRuleGroups, actualRuleGroups)

		if len(subDiff) == 0 {
			continue
		}

		diff[namespace] = subDiff
	}

	for namespace, actualRuleGroups := range actual {
		if seenNamespaces[namespace] {
			continue
		}

		subDiff := diffPrometheusRuleNamespaceState(nil, actualRuleGroups)

		diff[namespace] = subDiff
	}

	return diff
}

func diffPrometheusRuleNamespaceState(desired []rulefmt.RuleGroup, actual []rulefmt.RuleGroup) []PrometheusRuleGroupDiff {
	var diff []PrometheusRuleGroupDiff

	seenGroups := map[string]bool{}

desiredGroups:
	for _, desiredRuleGroup := range desired {
		seenGroups[desiredRuleGroup.Name] = true

		for _, actualRuleGroup := range actual {
			if desiredRuleGroup.Name == actualRuleGroup.Name {
				if equalPrometheusRuleGroups(desiredRuleGroup, actualRuleGroup) {
					continue desiredGroups
				}

				diff = append(diff, PrometheusRuleGroupDiff{
					Kind:    RuleGroupDiffKindUpdate,
					Actual:  actualRuleGroup,
					Desired: desiredRuleGroup,
				})
				continue desiredGroups
			}
		}

		diff = append(diff, PrometheusRuleGroupDiff{
			Kind:    RuleGroupDiffKindAdd,
			Desired: desiredRuleGroup,
		})
	}

	for _, actualRuleGroup := range actual {
		if seenGroups[actualRuleGroup.Name] {
			continue
		}

		diff = append(diff, PrometheusRuleGroupDiff{
			Kind:   RuleGroupDiffKindRemove,
			Actual: actualRuleGroup,
		})
	}

	return diff
}

func equalPrometheusRuleGroups(a, b rulefmt.RuleGroup) bool {
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
