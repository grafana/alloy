package pipelinetest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	promql_parser "github.com/prometheus/prometheus/promql/parser"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

// PrometheusAssertionSchema describes one declarative Prometheus assertion.
// Count assertions require Count and may optionally include match fields.
// Contains assertions require at least one match field.
type PrometheusAssertionSchema struct {
	// Sink, when set, checks only what the sink replacing this exit
	// component captured, such as loki.write.audit. It must be one of
	// config.sinks.
	Sink string `yaml:"sink,omitempty"`
	// Name optionally describes the assertion in reports.
	Name string `yaml:"name,omitempty"`
	// Type is count or contains. When unset, it is count if Count is set and
	// contains otherwise.
	Type  string                `yaml:"type,omitempty"`
	Count *int                  `yaml:"count,omitempty"`
	Match PrometheusMatchSchema `yaml:"match,omitempty"`
}

// PrometheusMatchSchema describes Prometheus sample fields used by declarative
// assertions. Match a metric by name through the __name__ label.
type PrometheusMatchSchema struct {
	// Selector is a PromQL series selector, such as
	// probe_success{job="blackbox", instance=~"prod-.*"}. The sample's labels
	// must satisfy every matcher, and may carry others.
	Selector  string                    `yaml:"selector,omitempty"`
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
		switch assertionType(assertion.Type, assertion.Count) {
		case assertionTypeCount:
			assert, err := buildPrometheusCountAssertion(assertion)
			if err != nil {
				return nil, err
			}
			out = append(out, assert)
		case assertionTypeContains:
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
	matchers := make([]harness.SampleMatcher, 0, 6)

	if match.Selector != "" {
		selector, err := harness.ParseSelector(match.Selector)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, harness.PrometheusSampleSelector(match.Selector, selector))
	}

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

// PrometheusInputSchema sends samples to Components, or, with Replace, where
// an entry component such as prometheus.scrape.default sent its samples. A
// replaced scrape doesn't add job and instance, so set them if needed.
type PrometheusInputSchema struct {
	Components []string                 `yaml:"components,omitempty"`
	Replace    string                   `yaml:"replace,omitempty"`
	Samples    []PrometheusSampleSchema `yaml:"samples"`
}

// PrometheusSampleSchema is one sample. Set Series in text exposition form,
// such as http_requests_total{code="200"}, or Labels including __name__.
type PrometheusSampleSchema struct {
	Series string            `yaml:"series,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
	Value  float64           `yaml:"value"`
	// Timestamp is RFC 3339. It defaults to the time the sample is sent.
	Timestamp string `yaml:"timestamp,omitempty"`
	// Type, Help and Unit are optional metadata sent with the sample.
	Type string `yaml:"type,omitempty"`
	Help string `yaml:"help,omitempty"`
	Unit string `yaml:"unit,omitempty"`
}

func buildPrometheusSamples(samples []PrometheusSampleSchema) ([]harness.InputSample, error) {
	out := make([]harness.InputSample, 0, len(samples))
	for i, sample := range samples {
		var ls labels.Labels
		switch {
		case sample.Series != "" && len(sample.Labels) > 0:
			return nil, fmt.Errorf("sample %d: set series or labels, not both", i)
		case sample.Series != "":
			parsed, err := promql_parser.NewParser(promql_parser.Options{}).ParseMetric(sample.Series)
			if err != nil {
				return nil, fmt.Errorf("sample %d: parse series %q: %w", i, sample.Series, err)
			}
			ls = parsed
		case len(sample.Labels) > 0:
			ls = toLabels(sample.Labels)
		default:
			return nil, fmt.Errorf("sample %d: series or labels is required", i)
		}
		if ls.Get(model.MetricNameLabel) == "" {
			return nil, fmt.Errorf("sample %d: the series needs a metric name", i)
		}

		var ts time.Time
		if sample.Timestamp != "" {
			parsed, err := time.Parse(time.RFC3339Nano, sample.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("sample %d: parse timestamp %q: %w", i, sample.Timestamp, err)
			}
			ts = parsed
		}

		out = append(out, harness.InputSample{
			Labels:    ls,
			Value:     sample.Value,
			Timestamp: ts,
			Metadata: metadata.Metadata{
				Type: model.MetricType(sample.Type),
				Help: sample.Help,
				Unit: sample.Unit,
			},
		})
	}
	return out, nil
}

func producePrometheusInputs(ctx context.Context, alloy *harness.Alloy, inputs []PrometheusInputSchema) error {
	for i, input := range inputs {
		samples, err := buildPrometheusSamples(input.Samples)
		if err != nil {
			return fmt.Errorf("prometheus input %d: %w", i, err)
		}
		if len(samples) == 0 {
			continue
		}
		source := harness.MustComponent[*harness.Source](alloy, generatedInputComponentID(signalMetrics, i))
		if err := source.SendSamples(ctx, samples...); err != nil {
			return fmt.Errorf("prometheus input %d: %w", i, err)
		}
	}
	return nil
}
