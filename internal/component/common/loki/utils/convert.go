package utils

import "github.com/prometheus/common/model"

// ToLabelSet converts a map of strings to a prometheus LabelSet.
func ToLabelSet(in map[string]string) model.LabelSet {
	res := make(model.LabelSet, len(in))
	for k, v := range in {
		res[model.LabelName(k)] = model.LabelValue(v)
	}
	return res
}
