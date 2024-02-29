package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// LabelSelector defines a selector to check to see if a set of Kubernetes
// labels matches a selector.
type LabelSelector struct {
	MatchLabels      map[string]string `river:"match_labels,attr,optional"`
	MatchExpressions []MatchExpression `river:"match_expression,block,optional"`
}

// BuildSelector builds a [labels.Selector] from a Flow LabelSelector.
func (ls *LabelSelector) BuildSelector() (labels.Selector, error) {
	if ls == nil {
		return metav1.LabelSelectorAsSelector(nil)
	}

	exprs := make([]metav1.LabelSelectorRequirement, 0, len(ls.MatchExpressions))
	for _, expr := range ls.MatchExpressions {
		exprs = append(exprs, expr.buildExpression())
	}

	return metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels:      ls.MatchLabels,
		MatchExpressions: exprs,
	})
}

// MatchExpression defines an expression matcher to check to see if some key
// from a Kubernetes resource matches a selector.
type MatchExpression struct {
	Key      string   `river:"key,attr"`
	Operator string   `river:"operator,attr"`
	Values   []string `river:"values,attr,optional"`
}

func (me *MatchExpression) buildExpression() metav1.LabelSelectorRequirement {
	if me == nil {
		return metav1.LabelSelectorRequirement{}
	}

	return metav1.LabelSelectorRequirement{
		Key:      me.Key,
		Operator: metav1.LabelSelectorOperator(me.Operator),
		Values:   me.Values,
	}
}
