package pipelinetest

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/pipelinetest/harness"
	"github.com/grafana/loki/pkg/push"
)

// TestSchema describes a declarative pipeline test loaded from a text file.
type TestSchema struct {
	Config ConfigSchema    `yaml:"config"`
	Inputs InputSchema     `yaml:"inputs"`
	Assert AssertionSchema `yaml:"assert"`
}

type ConfigSchema string

func (c *ConfigSchema) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*c = ConfigSchema(value.Value)
		return nil
	case yaml.MappingNode:
		var cfg struct {
			Path string `yaml:"path"`
		}
		if err := value.Decode(&cfg); err != nil {
			return err
		}
		if cfg.Path == "" {
			return errors.New("config mapping requires path")
		}

		bb, err := os.ReadFile(cfg.Path)
		if err != nil {
			return fmt.Errorf("read config path %q: %w", cfg.Path, err)
		}

		*c = ConfigSchema(bb)
		return nil
	default:
		return fmt.Errorf("config must be inline or from a path")
	}
}

// InputSchema groups pipeline test inputs by signal type.
type InputSchema struct {
	Loki []LokiInputSchema `yaml:"loki"`
}

// LokiInputSchema describes one Loki input, including the receiver targets to
// forward entries to and the entries to emit for the test.
type LokiInputSchema struct {
	Components []string          `yaml:"components"`
	Entries    []LokiEntrySchema `yaml:"entries,omitempty"`
}

// LokiEntrySchema describes one Loki entry in a declarative test file.
type LokiEntrySchema struct {
	Labels             map[string]string `yaml:"labels,omitempty"`
	Line               string            `yaml:"line,omitempty"`
	Timestamp          string            `yaml:"timestamp,omitempty"`
	StructuredMetadata map[string]string `yaml:"structured_metadata,omitempty"`
}

// AssertionSchema groups declarative assertions by signal type.
type AssertionSchema struct {
	Loki []LokiAssertionSchema `yaml:"loki"`
}

// LokiAssertionSchema describes one declarative Loki entry assertion. When
// Count is omitted, at least one matching entry must exist. When Count is set,
// exactly Count matching entries must exist. Mode controls how map-like fields
// such as labels and structured metadata are matched.
type LokiAssertionSchema struct {
	Type  string          `yaml:"type"`
	Count *int            `yaml:"count,omitempty"`
	Mode  string          `yaml:"mode,omitempty"`
	Match LokiMatchSchema `yaml:"match,omitempty"`
}

// LokiMatchSchema describes Loki entry fields used by declarative assertions.
type LokiMatchSchema struct {
	Labels             map[string]string `yaml:"labels,omitempty"`
	Line               string            `yaml:"line,omitempty"`
	Timestamp          string            `yaml:"timestamp,omitempty"`
	StructuredMetadata map[string]string `yaml:"structured_metadata,omitempty"`
}

// produceInputs sends all configured test inputs into the running pipeline.
func produceInputs(alloy *harness.Alloy, inputs InputSchema) error {
	return produceLokiInputs(alloy, inputs.Loki)
}

func produceLokiInputs(alloy *harness.Alloy, inputs []LokiInputSchema) error {
	for i, input := range inputs {
		if len(input.Entries) == 0 {
			continue
		}

		if len(input.Components) == 0 {
			return fmt.Errorf("loki input %d requires at least one receiver target", i)
		}

		source := harness.MustComponent[*harness.Source](alloy, generatedLokiInputComponentID(i))

		entries, err := buildLokiEntries(input.Entries)
		if err != nil {
			return fmt.Errorf("loki input %d: %w", i, err)
		}

		source.SendEntries(entries...)
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
	var timestamp time.Time
	if entry.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			return loki.Entry{}, fmt.Errorf("parse timestamp %q: %w", entry.Timestamp, err)
		}
		timestamp = parsed
	}

	return loki.NewEntry(
		toLabelSet(entry.Labels),
		push.Entry{
			Timestamp:          timestamp,
			Line:               entry.Line,
			StructuredMetadata: toLabelsAdapter(entry.StructuredMetadata),
		},
	), nil
}

// buildAssertions builds runtime assertions from the declarative schema.
func buildAssertions(assertions AssertionSchema) ([]harness.Assertion, error) {
	return buildLokiAssertions(assertions.Loki)
}

// buildLokiAssertions builds Loki assertions from the declarative schema.
func buildLokiAssertions(assertions []LokiAssertionSchema) ([]harness.Assertion, error) {
	out := make([]harness.Assertion, 0, len(assertions))
	for _, assertion := range assertions {
		switch assertion.Type {
		case "entry":
			built, err := buildLokiEntryAssertion(assertion)
			if err != nil {
				return nil, err
			}
			out = append(out, built)
		default:
			return nil, fmt.Errorf("unknown assertion type %q", assertion.Type)
		}
	}
	return out, nil
}

func buildLokiEntryAssertion(assertion LokiAssertionSchema) (harness.Assertion, error) {
	matchers, err := buildLokiMatchers(assertion.Match, assertion.Mode)
	if err != nil {
		return nil, err
	}

	if assertion.Count == nil && len(matchers) == 0 {
		return nil, errors.New("entry requires count or at least one match field")
	}

	return harness.LokiEntries(assertion.Count, matchers...), nil
}

func buildLokiMatchers(match LokiMatchSchema, mode string) ([]harness.EntryMatcher, error) {
	matchers := make([]harness.EntryMatcher, 0, 4)

	if len(match.Labels) > 0 {
		matchers = append(matchers, harness.LokiEntryLabels(toLabelSet(match.Labels), mode == "partial"))
	}

	if match.Line != "" {
		matchers = append(matchers, harness.LokiEntryLine(match.Line))
	}

	if match.Timestamp != "" {
		// FIXME(kalleep): We need to be able to configure layout.
		parsed, err := time.Parse(time.RFC3339Nano, match.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse match timestamp %q: %w", match.Timestamp, err)
		}
		matchers = append(matchers, harness.LokiEntryTimestamp(parsed))
	}

	if len(match.StructuredMetadata) > 0 {
		matchers = append(matchers, harness.LokiEntryStructuredMetadata(toLabelsAdapter(match.StructuredMetadata), mode == "partial"))
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
