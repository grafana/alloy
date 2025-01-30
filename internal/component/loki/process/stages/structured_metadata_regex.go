package stages

import (
	"github.com/go-kit/log"
	"regexp"

	"github.com/grafana/loki/v3/pkg/logproto"
)

type StructuredMetadataRegexConfig struct {
	Regex string `alloy:"regex,attr"`
}

func newStructuredMetadataRegexStage(logger log.Logger, configs StructuredMetadataRegexConfig) (Stage, error) {

	re, error := regexp.Compile(configs.Regex)

	if error != nil {
		return &structuredMetadataRegexStage{}, error
	}

	return &structuredMetadataRegexStage{
		logger: logger,
		regex:  *re,
	}, nil
}

type structuredMetadataRegexStage struct {
	logger log.Logger
	regex  regexp.Regexp
}

func (s *structuredMetadataRegexStage) Name() string {
	return StageTypeStructuredMetadataRegex
}

// Cleanup implements Stage.
func (*structuredMetadataRegexStage) Cleanup() {
	// no-op
}

func (s *structuredMetadataRegexStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		for labelName, labelValue := range e.Labels {
			if s.regex.MatchString(string(labelName)) {
				e.StructuredMetadata = append(e.StructuredMetadata, logproto.LabelAdapter{Name: string(labelName), Value: string(labelValue)})
				delete(e.Labels, labelName)
			}
		}
		return e
	})
}
