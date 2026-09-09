package pipelinetest

import (
	"errors"
	"fmt"
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
	Labels    MapMatchSchema `yaml:"labels,omitempty"`
	Value     *float64       `yaml:"value,omitempty"`
	Timestamp string         `yaml:"timestamp,omitempty"`
	Histogram *bool          `yaml:"histogram,omitempty"`
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

func buildPrometheusMatchers(match PrometheusMatchSchema) ([]harness.SampleMatcher, error) {
	matchers := make([]harness.SampleMatcher, 0, 4)

	if len(match.Labels.Values) > 0 {
		partial, err := isPartialMapMatch("labels", match.Labels)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, harness.PrometheusSampleLabels(toLabels(match.Labels.Values), partial))
	}

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
