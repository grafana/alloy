package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type RuleGroupDiffKind string

const (
	RuleGroupDiffKindAdd    RuleGroupDiffKind = "add"
	RuleGroupDiffKindRemove RuleGroupDiffKind = "remove"
	RuleGroupDiffKindUpdate RuleGroupDiffKind = "update"
)

type LabelSelector struct {
	MatchLabels      map[string]string `alloy:"match_labels,attr,optional"`
	MatchExpressions []MatchExpression `alloy:"match_expression,block,optional"`
}

type MatchExpression struct {
	Key      string   `alloy:"key,attr"`
	Operator string   `alloy:"operator,attr"`
	Values   []string `alloy:"values,attr,optional"`
}

func ConvertSelectorToListOptions(selector LabelSelector) (labels.Selector, error) {
	matchExpressions := []metav1.LabelSelectorRequirement{}

	for _, me := range selector.MatchExpressions {
		matchExpressions = append(matchExpressions, metav1.LabelSelectorRequirement{
			Key:      me.Key,
			Operator: metav1.LabelSelectorOperator(me.Operator),
			Values:   me.Values,
		})
	}

	return metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels:      selector.MatchLabels,
		MatchExpressions: matchExpressions,
	})
}
