package podlogs

import (
	"strings"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
)

func convertRelabelConfig(in []*promv1.RelabelConfig) ([]*relabel.Config, error) {
	res := make([]*relabel.Config, 0, len(in))

	for _, inRule := range in {
		outRule := relabel.DefaultRelabelConfig
		if len(inRule.SourceLabels) > 0 {
			outRule.SourceLabels = convertLabelNames(inRule.SourceLabels)
		}
		if inRule.Separator != "" {
			outRule.Separator = inRule.Separator
		}
		if inRule.Regex != "" {
			regex, err := relabel.NewRegexp(inRule.Regex)
			if err != nil {
				return nil, err
			}
			outRule.Regex = regex
		}
		if inRule.Modulus != 0 {
			outRule.Modulus = inRule.Modulus
		}
		if inRule.TargetLabel != "" {
			outRule.TargetLabel = inRule.TargetLabel
		}
		if inRule.Replacement != "" {
			outRule.Replacement = inRule.Replacement
		}
		if inRule.Action != "" {
			outRule.Action = relabel.Action(strings.ToLower(inRule.Action))
		}

		res = append(res, &outRule)
	}

	return res, nil
}

func convertLabelNames(in []promv1.LabelName) model.LabelNames {
	res := make([]model.LabelName, 0, len(in))
	for _, inName := range in {
		res = append(res, model.LabelName(inName))
	}
	return res
}
