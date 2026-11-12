package pipelinetest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/pipelinetest/harness"
	"github.com/grafana/loki/pkg/push"
)

// LokiInputSchema describes one Loki input, including the receiver targets to
// forward entries to and the entries to emit for the test.
type LokiInputSchema struct {
	Components []string `yaml:"components,omitempty"`
	// Replace names an entry component of the config, such as
	// loki.source.file.app. It is removed, and the entries are sent wherever
	// its forward_to sent them. Set Components or Replace, not both.
	Replace string            `yaml:"replace,omitempty"`
	Entries []LokiEntrySchema `yaml:"entries,omitempty"`
}

// LokiEntrySchema describes one Loki entry in a declarative test file.
type LokiEntrySchema struct {
	Labels map[string]string `yaml:"labels,omitempty"`
	Line   string            `yaml:"line,omitempty"`
	// LineRepeat, when set, repeats Line that many times, so a test can send
	// a long line without a large file.
	LineRepeat int `yaml:"line_repeat,omitempty"`
	// Timestamp is RFC 3339. It defaults to the time the entry is sent.
	Timestamp          string            `yaml:"timestamp,omitempty"`
	StructuredMetadata map[string]string `yaml:"structured_metadata,omitempty"`
}

// LokiAssertionSchema describes one declarative Loki assertion. Count
// assertions require Count and may optionally include match fields. Contains
// assertions require at least one match field.
type LokiAssertionSchema struct {
	// Sink, when set, checks only what the sink replacing this exit
	// component captured, such as loki.write.audit. It must be one of
	// config.sinks.
	Sink string `yaml:"sink,omitempty"`
	// Name optionally describes the assertion in reports.
	Name string `yaml:"name,omitempty"`
	// Type is count or contains. When unset, it is count if Count is set and
	// contains otherwise.
	Type  string          `yaml:"type,omitempty"`
	Count *int            `yaml:"count,omitempty"`
	Match LokiMatchSchema `yaml:"match,omitempty"`
}

// LokiMatchSchema describes Loki entry fields used by declarative assertions.
type LokiMatchSchema struct {
	// Selector is a LogQL-style stream selector, such as
	// {service_name="api", level=~"warn|error"}. The entry's labels must
	// satisfy every matcher, and may carry others.
	Selector string `yaml:"selector,omitempty"`
	Line     string `yaml:"line,omitempty"`
	// LineRegex matches the line anywhere, as a Go regular expression. Use
	// it for lines with parts that vary, such as hashes.
	LineRegex          string         `yaml:"line_regex,omitempty"`
	Timestamp          string         `yaml:"timestamp,omitempty"`
	Labels             MapMatchSchema `yaml:"labels,omitempty"`
	StructuredMetadata MapMatchSchema `yaml:"structured_metadata,omitempty"`
}

func produceLokiInputs(ctx context.Context, alloy *harness.Alloy, ids inputComponents, inputs []LokiInputSchema) error {
	for i, input := range inputs {
		if len(input.Entries) == 0 {
			continue
		}

		sources, err := ids.sources(alloy, signalLogs, i)
		if err != nil {
			return err
		}

		entries, err := buildLokiEntries(input.Entries)
		if err != nil {
			return fmt.Errorf("loki input %d: %w", i, err)
		}

		for _, source := range sources {
			if err := source.SendEntries(ctx, entries...); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildLokiEntries(entries []LokiEntrySchema) ([]loki.Entry, error) {
	out := make([]loki.Entry, 0, len(entries))
	for _, entry := range entries {
		parsed, err := buildLokiEntry(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func buildLokiEntry(entry LokiEntrySchema) (loki.Entry, error) {
	// Like a real source, stamp entries with the time they are read.
	timestamp := time.Now()
	if entry.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			return loki.Entry{}, fmt.Errorf("parse timestamp %q: %w", entry.Timestamp, err)
		}
		timestamp = parsed
	}

	line := entry.Line
	switch {
	case entry.LineRepeat < 0:
		return loki.Entry{}, fmt.Errorf("line_repeat must be positive, got %d", entry.LineRepeat)
	case entry.LineRepeat > 0:
		if line == "" {
			return loki.Entry{}, errors.New("line_repeat needs a line to repeat")
		}
		line = strings.Repeat(line, entry.LineRepeat)
	}

	return loki.NewEntry(
		toLabelSet(entry.Labels),
		push.Entry{
			Timestamp:          timestamp,
			Line:               line,
			StructuredMetadata: toLabelsAdapter(entry.StructuredMetadata),
		},
	), nil
}

func buildLokiAssertions(assertions []LokiAssertionSchema) ([]harness.Assertion, error) {
	out := make([]harness.Assertion, 0, len(assertions))
	for _, assertion := range assertions {
		switch assertionType(assertion.Type, assertion.Count) {
		case assertionTypeCount:
			assert, err := buildLokiCountAssertion(assertion)
			if err != nil {
				return nil, err
			}
			out = append(out, assert)
		case assertionTypeContains:
			assert, err := buildLokiContainsAssertion(assertion)
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

func buildLokiCountAssertion(assertion LokiAssertionSchema) (harness.Assertion, error) {
	matchers, err := buildLokiMatchers(assertion.Match)
	if err != nil {
		return nil, err
	}

	if assertion.Count == nil {
		return nil, errors.New("count requires count")
	}

	return harness.LokiEntryCount(*assertion.Count, matchers...), nil
}

func buildLokiContainsAssertion(assertion LokiAssertionSchema) (harness.Assertion, error) {
	matchers, err := buildLokiMatchers(assertion.Match)
	if err != nil {
		return nil, err
	}

	if len(matchers) == 0 {
		return nil, errors.New("contains requires at least one match field")
	}

	return harness.LokiContainsEntry(matchers...), nil
}

func buildLokiMatchers(match LokiMatchSchema) ([]harness.EntryMatcher, error) {
	matchers := make([]harness.EntryMatcher, 0, 5)

	if match.Selector != "" {
		selector, err := harness.ParseSelector(match.Selector)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, harness.LokiEntrySelector(match.Selector, selector))
	}

	if len(match.Labels.Values) > 0 {
		partial, err := isPartialMapMatch("labels", match.Labels)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, harness.LokiEntryLabels(toLabelSet(match.Labels.Values), partial))
	}

	if match.Line != "" {
		matchers = append(matchers, harness.LokiEntryLine(match.Line))
	}

	if match.LineRegex != "" {
		re, err := regexp.Compile(match.LineRegex)
		if err != nil {
			return nil, fmt.Errorf("parse line_regex %q: %w", match.LineRegex, err)
		}
		matchers = append(matchers, harness.LokiEntryLineRegex(re))
	}

	if match.Timestamp != "" {
		// FIXME(kalleep): We need to be able to configure layout.
		parsed, err := time.Parse(time.RFC3339Nano, match.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse match timestamp %q: %w", match.Timestamp, err)
		}
		matchers = append(matchers, harness.LokiEntryTimestamp(parsed))
	}

	if len(match.StructuredMetadata.Values) > 0 {
		partial, err := isPartialMapMatch("structured_metadata", match.StructuredMetadata)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, harness.LokiEntryStructuredMetadata(toLabelsAdapter(match.StructuredMetadata.Values), partial))
	}

	return matchers, nil
}

func toLabelSet(labels map[string]string) model.LabelSet {
	if len(labels) == 0 {
		return nil
	}

	out := make(model.LabelSet, len(labels))
	for k, v := range labels {
		out[model.LabelName(k)] = model.LabelValue(v)
	}
	return out
}

func toLabelsAdapter(labels map[string]string) push.LabelsAdapter {
	if len(labels) == 0 {
		return nil
	}

	out := make(push.LabelsAdapter, 0, len(labels))
	for name, value := range labels {
		out = append(out, push.LabelAdapter{
			Name:  name,
			Value: value,
		})
	}
	return out
}
