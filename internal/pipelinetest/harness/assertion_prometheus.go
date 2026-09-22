package harness

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/prometheus/model/labels"
)

// PrometheusSampleCount returns an Assertion that passes when exactly want
// Prometheus samples match all provided matchers. When no matchers are
// provided, all samples are counted.
func PrometheusSampleCount(want int, matchers ...SampleMatcher) Assertion {
	return func(s snapshot) error {
		var got int
		for _, sample := range s.prometheus {
			if sampleMatches(sample, matchers...) {
				got++
			}
		}

		if got == want {
			return nil
		}

		message := fmt.Sprintf("want %d, got %d", want, got)
		if conditions := sampleMatcherConditions(matchers); len(conditions) > 0 {
			message += " for " + conditions
		}

		return AssertionError{
			Kind:    "prometheus.count",
			Message: message,
		}
	}
}

// PrometheusContainsSample returns an Assertion that passes when the snapshot
// contains at least one Prometheus sample matched by all provided matchers.
func PrometheusContainsSample(matchers ...SampleMatcher) Assertion {
	return func(s snapshot) error {
		for _, sample := range s.prometheus {
			if sampleMatches(sample, matchers...) {
				return nil
			}
		}

		message := "no matching sample found"
		if conditions := sampleMatcherConditions(matchers); len(conditions) > 0 {
			message += " for " + conditions
		}

		return AssertionError{
			Kind:    "prometheus.contains",
			Message: message,
		}
	}
}

type SampleMatcher struct {
	match func(sample PrometheusSample) bool
	text  string
}

// PrometheusSampleLabels returns a SampleMatcher for sample labels. When
// partial is false, labels must match exactly. When partial is true, the sample
// labels must contain at least the provided labels.
func PrometheusSampleLabels(want labels.Labels, partial bool) SampleMatcher {
	var match func(sample PrometheusSample) bool
	if partial {
		match = func(sample PrometheusSample) bool {
			return labelsContain(sample.Labels, want)
		}
	} else {
		match = func(sample PrometheusSample) bool {
			return labels.Equal(sample.Labels, want)
		}
	}

	return SampleMatcher{
		match: match,
		text:  renderPrometheusLabels(want),
	}
}

// PrometheusSampleValue returns a SampleMatcher that matches the sample value
// exactly. Native histogram samples never match, as they carry no float value.
func PrometheusSampleValue(value float64) SampleMatcher {
	return SampleMatcher{
		match: func(sample PrometheusSample) bool {
			return sample.Histogram == nil && sample.Value == value
		},
		text: renderValue(value),
	}
}

// PrometheusSampleTimestamp returns a SampleMatcher that matches the sample
// timestamp exactly.
func PrometheusSampleTimestamp(ts time.Time) SampleMatcher {
	return SampleMatcher{
		match: func(sample PrometheusSample) bool {
			return sample.Timestamp.Equal(ts)
		},
		text: renderTimestamp(ts),
	}
}

// PrometheusSampleIsHistogram returns a SampleMatcher that matches samples
// carrying a native histogram, or samples carrying a float value when want is
// false.
func PrometheusSampleIsHistogram(want bool) SampleMatcher {
	return SampleMatcher{
		match: func(sample PrometheusSample) bool {
			return (sample.Histogram != nil) == want
		},
		text: "histogram = " + strconv.FormatBool(want),
	}
}

func sampleMatches(sample PrometheusSample, matchers ...SampleMatcher) bool {
	for _, matcher := range matchers {
		if !matcher.match(sample) {
			return false
		}
	}
	return true
}

func sampleMatcherConditions(matchers []SampleMatcher) string {
	conditions := make([]string, 0, len(matchers))
	for _, matcher := range matchers {
		if matcher.text == "" {
			continue
		}
		conditions = append(conditions, matcher.text)
	}
	return strings.Join(conditions, ", ")
}

func renderPrometheusSamples(samples []PrometheusSample) string {
	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "prometheus samples (%d):\n", len(samples))
	for _, sample := range samples {
		builder.WriteString("- ")
		builder.WriteString(renderPrometheusSample(sample))
		builder.WriteByte('\n')
	}

	return strings.TrimSuffix(builder.String(), "\n")
}

func renderPrometheusSample(sample PrometheusSample) string {
	parts := []string{
		renderPrometheusLabels(sample.Labels),
		renderTimestamp(sample.Timestamp),
	}
	if sample.Histogram != nil {
		parts = append(parts, "histogram = true")
	} else {
		parts = append(parts, renderValue(sample.Value))
	}
	return strings.Join(parts, " ")
}

func renderPrometheusLabels(ls labels.Labels) string {
	if ls.IsEmpty() {
		return "labels = {}"
	}

	parts := make([]string, 0, ls.Len())
	ls.Range(func(label labels.Label) {
		parts = append(parts, fmt.Sprintf("%s=%q", label.Name, label.Value))
	})

	return "labels = {" + strings.Join(parts, ", ") + "}"
}

func renderValue(value float64) string {
	return "value = " + strconv.FormatFloat(value, 'g', -1, 64)
}

func labelsContain(got, want labels.Labels) bool {
	contains := true
	want.Range(func(label labels.Label) {
		if got.Get(label.Name) != label.Value {
			contains = false
		}
	})
	return contains
}
