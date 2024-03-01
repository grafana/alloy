package rules

import "fmt"

type DebugInfo struct {
	Error               string                   `river:"error,attr,optional"`
	PrometheusRules     []DebugK8sPrometheusRule `river:"prometheus_rule,block,optional"`
	MimirRuleNamespaces []DebugMimirNamespace    `river:"mimir_rule_namespace,block,optional"`
}

type DebugK8sPrometheusRule struct {
	Namespace     string `river:"namespace,attr"`
	Name          string `river:"name,attr"`
	UID           string `river:"uid,attr"`
	NumRuleGroups int    `river:"num_rule_groups,attr"`
}

type DebugMimirNamespace struct {
	Name          string `river:"name,attr"`
	NumRuleGroups int    `river:"num_rule_groups,attr"`
}

func (c *Component) DebugInfo() interface{} {
	var output DebugInfo
	for ns := range c.currentState {
		if !isManagedMimirNamespace(c.args.MimirNameSpacePrefix, ns) {
			continue
		}

		output.MimirRuleNamespaces = append(output.MimirRuleNamespaces, DebugMimirNamespace{
			Name:          ns,
			NumRuleGroups: len(c.currentState[ns]),
		})
	}

	// This should load from the informer cache, so it shouldn't fail under normal circumstances.
	managedK8sNamespaces, err := c.namespaceLister.List(c.namespaceSelector)
	if err != nil {
		return DebugInfo{
			Error: fmt.Sprintf("failed to list namespaces: %v", err),
		}
	}

	for _, n := range managedK8sNamespaces {
		// This should load from the informer cache, so it shouldn't fail under normal circumstances.
		rules, err := c.ruleLister.PrometheusRules(n.Name).List(c.ruleSelector)
		if err != nil {
			return DebugInfo{
				Error: fmt.Sprintf("failed to list rules: %v", err),
			}
		}

		for _, r := range rules {
			output.PrometheusRules = append(output.PrometheusRules, DebugK8sPrometheusRule{
				Namespace:     n.Name,
				Name:          r.Name,
				UID:           string(r.UID),
				NumRuleGroups: len(r.Spec.Groups),
			})
		}
	}

	return output
}
