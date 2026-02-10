package rules

import "fmt"

type DebugInfo struct {
	Error              string                   `alloy:"error,attr,optional"`
	PrometheusRules    []DebugK8sPrometheusRule `alloy:"prometheus_rule,block,optional"`
	LokiRuleNamespaces []DebugLokiNamespace     `alloy:"loki_rule_namespace,block,optional"`
}

type DebugK8sPrometheusRule struct {
	Namespace     string `alloy:"namespace,attr"`
	Name          string `alloy:"name,attr"`
	UID           string `alloy:"uid,attr"`
	NumRuleGroups int    `alloy:"num_rule_groups,attr"`
}

type DebugLokiNamespace struct {
	Name          string `alloy:"name,attr"`
	NumRuleGroups int    `alloy:"num_rule_groups,attr"`
}

func (c *Component) DebugInfo() any {
	var output DebugInfo
	for ns := range c.currentState {
		if !isManagedLokiNamespace(c.args.LokiNameSpacePrefix, ns) {
			continue
		}

		output.LokiRuleNamespaces = append(output.LokiRuleNamespaces, DebugLokiNamespace{
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
