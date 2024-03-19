package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type LabelSelector struct {
	MatchLabels      map[string]string `river:"match_labels,attr,optional"`
	MatchExpressions []MatchExpression `river:"match_expression,block,optional"`
}

type MatchExpression struct {
	Key      string   `river:"key,attr"`
	Operator string   `river:"operator,attr"`
	Values   []string `river:"values,attr,optional"`
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
