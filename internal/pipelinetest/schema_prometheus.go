package pipelinetest

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

// PrometheusAssertionSchema describes one declarative Prometheus assertion.
// Count assertions require Count and may optionally include match fields.
// Contains assertions require at least one match field.
type PrometheusAssertionSchema struct {
	Type  string                `yaml:"type"`
	Count *int                  `yaml:"count,omitempty"`
	Match PrometheusMatchSchema `yaml:"match,omitempty"`
}

// PrometheusMatchSchema describes Prometheus sample fields used by declarative
// assertions. Match a metric by name through the __name__ label.
type PrometheusMatchSchema struct {
	Labels    MapMatchSchema            `yaml:"labels,omitempty"`
	Value     *float64                  `yaml:"value,omitempty"`
	Timestamp string                    `yaml:"timestamp,omitempty"`
	Histogram *bool                     `yaml:"histogram,omitempty"`
	Metadata  *PrometheusMetadataSchema `yaml:"metadata,omitempty"`
}

// PrometheusMetadataSchema describes the series metadata a sample must carry.
// Omitted fields are not checked.
type PrometheusMetadataSchema struct {
	Type *string `yaml:"type,omitempty"`
	Unit *string `yaml:"unit,omitempty"`
	Help *string `yaml:"help,omitempty"`
}

func buildPrometheusAssertions(assertions []PrometheusAssertionSchema) ([]harness.Assertion, error) {
	out := make([]harness.Assertion, 0, len(assertions))
	for _, assertion := range assertions {
		switch assertion.Type {
		case "count":
			assert, err := buildPrometheusCountAssertion(assertion)
			if err != nil {
				return nil, err
			}
			out = append(out, assert)
		case "contains":
			assert, err := buildPrometheusContainsAssertion(assertion)
			if err != nil {
				return nil, err
			}
			out = append(out, assert)
		default:
			return nil, fmt.Errorf("unknown assertion type %q", assertion.Type)
		}
	}
	return out, nil
}

func buildPrometheusCountAssertion(assertion PrometheusAssertionSchema) (harness.Assertion, error) {
	matchers, err := buildPrometheusMatchers(assertion.Match)
	if err != nil {
		return nil, err
	}

	if assertion.Count == nil {
		return nil, errors.New("count requires count")
	}

	return harness.PrometheusSampleCount(*assertion.Count, matchers...), nil
}

func buildPrometheusContainsAssertion(assertion PrometheusAssertionSchema) (harness.Assertion, error) {
	matchers, err := buildPrometheusMatchers(assertion.Match)
	if err != nil {
		return nil, err
	}

	if len(matchers) == 0 {
		return nil, errors.New("contains requires at least one match field")
	}

	return harness.PrometheusContainsSample(matchers...), nil
}

// buildPrometheusLabelMatchers builds the matchers for literal and regex
// labels. In exact mode, the sample must have exactly the labels that the
// literal values and the regexes name together.
func buildPrometheusLabelMatchers(match MapMatchSchema) ([]harness.SampleMatcher, error) {
	if len(match.Values) == 0 && len(match.Regex) == 0 {
		return nil, nil
	}
	partial, err := isPartialMapMatch("labels", match)
	if err != nil {
		return nil, err
	}
	if len(match.Regex) == 0 {
		return []harness.SampleMatcher{harness.PrometheusSampleLabels(toLabels(match.Values), partial)}, nil
	}

	var matchers []harness.SampleMatcher
	if len(match.Values) > 0 {
		matchers = append(matchers, harness.PrometheusSampleLabels(toLabels(match.Values), true))
	}
	for _, name := range slices.Sorted(maps.Keys(match.Regex)) {
		matcher, err := harness.PrometheusSampleLabelRegex(name, match.Regex[name])
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, matcher)
	}
	if !partial {
		names := slices.Concat(slices.Collect(maps.Keys(match.Values)), slices.Collect(maps.Keys(match.Regex)))
		slices.Sort(names)
		matchers = append(matchers, harness.PrometheusSampleLabelNames(slices.Compact(names)))
	}
	return matchers, nil
}

func buildPrometheusMatchers(match PrometheusMatchSchema) ([]harness.SampleMatcher, error) {
	matchers := make([]harness.SampleMatcher, 0, 5)

	labelMatchers, err := buildPrometheusLabelMatchers(match.Labels)
	if err != nil {
		return nil, err
	}
	matchers = append(matchers, labelMatchers...)

	if match.Value != nil {
		matchers = append(matchers, harness.PrometheusSampleValue(*match.Value))
	}

	if match.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339Nano, match.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse match timestamp %q: %w", match.Timestamp, err)
		}
		matchers = append(matchers, harness.PrometheusSampleTimestamp(parsed))
	}

	if match.Histogram != nil {
		matchers = append(matchers, harness.PrometheusSampleIsHistogram(*match.Histogram))
	}

	if match.Metadata != nil {
		matchers = append(matchers, harness.PrometheusSampleMetadata(match.Metadata.Type, match.Metadata.Unit, match.Metadata.Help))
	}

	return matchers, nil
}

func toLabels(values map[string]string) labels.Labels {
	if len(values) == 0 {
		return labels.EmptyLabels()
	}

	builder := labels.NewScratchBuilder(len(values))
	for name, value := range values {
		builder.Add(name, value)
	}
	builder.Sort()
	return builder.Labels()
}
