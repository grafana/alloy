package harness

import (
	"fmt"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	promql_parser "github.com/prometheus/prometheus/promql/parser"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// ParseSelector parses a PromQL series selector, such as
// probe_success{job="blackbox"}, or a Loki stream selector, which has the same
// syntax. A series matches when every matcher does, so it can carry other
// labels.
func ParseSelector(selector string) ([]*labels.Matcher, error) {
	matchers, err := promql_parser.NewParser(promql_parser.Options{}).ParseMetricSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("parse selector %q: %w", selector, err)
	}
	return matchers, nil
}

// PrometheusSampleSelector matches samples whose labels satisfy every matcher.
func PrometheusSampleSelector(selector string, matchers []*labels.Matcher) SampleMatcher {
	return SampleMatcher{
		match: func(sample PrometheusSample) bool {
			for _, m := range matchers {
				if !m.Matches(sample.Labels.Get(m.Name)) {
					return false
				}
			}
			return true
		},
		text: "selector " + selector,
	}
}

// LokiEntrySelector matches entries whose labels satisfy every matcher.
func LokiEntrySelector(selector string, matchers []*labels.Matcher) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			for _, m := range matchers {
				if !m.Matches(string(entry.Labels[model.LabelName(m.Name)])) {
					return false
				}
			}
			return true
		},
		text: "selector " + selector,
	}
}
