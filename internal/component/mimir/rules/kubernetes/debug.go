package rules

import "fmt"

type DebugInfo struct {
	Error               string                   `alloy:"error,attr,optional"`
	PrometheusRules     []DebugK8sPrometheusRule `alloy:"prometheus_rule,block,optional"`
	MimirRuleNamespaces []DebugMimirNamespace    `alloy:"mimir_rule_namespace,block,optional"`
}

type DebugK8sPrometheusRule struct {
	Namespace     string `alloy:"namespace,attr"`
	Name          string `alloy:"name,attr"`
	UID           string `alloy:"uid,attr"`
	NumRuleGroups int    `alloy:"num_rule_groups,attr"`
}

type DebugMimirNamespace struct {
	Name          string `alloy:"name,attr"`
	NumRuleGroups int    `alloy:"num_rule_groups,attr"`
}

func (c *Component) DebugInfo() any {
	var output DebugInfo

	if c.eventProcessor == nil {
		return output
	}

	currentState := c.eventProcessor.getMimirState()
	for namespace := range currentState {
		if !isManagedMimirNamespace(c.args.MimirNameSpacePrefix, namespace) {
			continue
		}

		output.MimirRuleNamespaces = append(output.MimirRuleNamespaces, DebugMimirNamespace{
			Name:          namespace,
			NumRuleGroups: len(currentState[namespace]),
		})
	}

	// This should load from the informer cache, so it shouldn't fail under normal circumstances.
	rulesByNamespace, err := c.eventProcessor.getKubernetesState()
	if err != nil {
		return DebugInfo{Error: fmt.Sprintf("failed to list rules: %v", err)}
	}

	for namespace, rules := range rulesByNamespace {
		for _, rule := range rules {
			output.PrometheusRules = append(output.PrometheusRules, DebugK8sPrometheusRule{
				Namespace:     namespace,
				Name:          rule.Name,
				UID:           string(rule.UID),
				NumRuleGroups: len(rule.Spec.Groups),
			})
		}
	}

	return output
}
