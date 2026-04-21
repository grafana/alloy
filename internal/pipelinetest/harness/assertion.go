package harness

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

type Assertion func(s snapshot) error

type AssertionError struct {
	Kind    string
	Message string
}

func (e AssertionError) Error() string {
	if e.Kind == "" {
		return e.Message
	}
	return e.Kind + ": " + e.Message
}

type AssertionErrors struct {
	Errors   []error
	Snapshot snapshot
}

func (e AssertionErrors) Error() string {
	var builder strings.Builder
	builder.WriteString("pipeline test failed\n\n")

	for _, err := range e.Errors {
		builder.WriteString("- ")
		builder.WriteString(err.Error())
		builder.WriteByte('\n')
	}

	builder.WriteString("\nlatest snapshot:\n")
	builder.WriteString(renderSnapshot(e.Snapshot))

	return strings.TrimSuffix(builder.String(), "\n")
}

// LokiEntryMatch returns an Assertion that passes when the snapshot contains
// at least one Loki entry exactly matching want.
func LokiEntryMatch(want loki.Entry) Assertion {
	return LokiHasEntry(
		LokiEntryLabels(want.Labels),
		LokiEntryTimestamp(want.Timestamp),
		LokiEntryLine(want.Line),
		LokiEntryStructuredMetadata(want.StructuredMetadata),
	)
}

// LokiEntryCount returns an Assertion that passes when the snapshot contains
// exactly want Loki entries.
func LokiEntryCount(want int) Assertion {
	return func(s snapshot) error {
		if want != len(s.loki) {
			return AssertionError{
				Kind:    "loki.entry_count",
				Message: fmt.Sprintf("want %d, got %d", want, len(s.loki)),
			}
		}
		return nil
	}
}

// LokiHasEntry returns an Assertion that passes when the snapshot contains
// at least one Loki entry matched by all provided matchers.
func LokiHasEntry(matchers ...EntryMatcher) Assertion {
	return func(s snapshot) error {
		for _, entry := range s.loki {
			matched := true
			for _, matcher := range matchers {
				if !matcher.match(entry) {
					matched = false
					break
				}
			}
			if matched {
				return nil
			}
		}

		conditions := make([]string, 0, len(matchers))
		for _, matcher := range matchers {
			if matcher.text == "" {
				continue
			}
			conditions = append(conditions, matcher.text)
		}

		message := "no matching entry found"
		if len(conditions) > 0 {
			message += " for " + strings.Join(conditions, ", ")
		}

		return AssertionError{
			Kind:    "loki.has_entry",
			Message: message,
		}
	}
}

type EntryMatcher struct {
	match func(entry loki.Entry) bool
	text  string
}

// LokiEntryLine returns an EntryMatcher that matches the Loki entry line
// exactly.
func LokiEntryLine(line string) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			return entry.Line == line
		},
		text: renderLine(line),
	}
}

// LokiEntryLabels returns an EntryMatcher that matches the Loki entry labels
// exactly.
func LokiEntryLabels(labels model.LabelSet) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			return reflect.DeepEqual(entry.Labels, labels)
		},
		text: renderLabelSet(labels),
	}
}

// LokiEntryStructuredMetadata returns an EntryMatcher that matches the Loki
// entry structured metadata.
func LokiEntryStructuredMetadata(metadata push.LabelsAdapter) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			return structuredMetadataEqual(entry.StructuredMetadata, metadata)
		},
		text: renderStructuredMetadata(metadata),
	}
}

// LokiEntryTimestamp returns an EntryMatcher that matches the Loki entry
// timestamp exactly.
func LokiEntryTimestamp(ts time.Time) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			return entry.Timestamp.Equal(ts)
		},
		text: renderTimestamp(ts),
	}
}

func renderSnapshot(s snapshot) string {
	if len(s.loki) == 0 {
		return "loki entries (0)"
	}

	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "loki entries (%d):\n", len(s.loki))
	for _, entry := range s.loki {
		builder.WriteString("- ")
		builder.WriteString(renderLokiEntry(entry))
		builder.WriteByte('\n')
	}

	return strings.TrimSuffix(builder.String(), "\n")
}

func renderLokiEntry(entry loki.Entry) string {
	parts := []string{
		renderLabelSet(entry.Labels),
		renderLine(entry.Line),
		renderTimestamp(entry.Timestamp),
		renderStructuredMetadata(entry.StructuredMetadata),
	}
	return strings.Join(parts, " ")
}

func renderLine(line string) string {
	return fmt.Sprintf("line = %q", line)
}

func renderTimestamp(timestamp time.Time) string {
	return "timestamp = " + timestamp.Format(time.RFC3339Nano)
}

func renderLabelSet(labels model.LabelSet) string {
	if len(labels) == 0 {
		return "labels = {}"
	}

	keys := make([]string, 0, len(labels))
	for name := range labels {
		keys = append(keys, string(name))
	}
	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s=%q`, key, labels[model.LabelName(key)]))
	}

	return "labels = {" + strings.Join(parts, ", ") + "}"
}

func renderStructuredMetadata(labels push.LabelsAdapter) string {
	if len(labels) == 0 {
		return "structured_metadata = {}"
	}

	parts := make([]string, 0, len(labels))
	for _, label := range sortStructuredMetadata(labels) {
		parts = append(parts, fmt.Sprintf(`%s=%q`, label.Name, label.Value))
	}

	return "structured_metadata = {" + strings.Join(parts, ", ") + "}"
}

func structuredMetadataEqual(got, want push.LabelsAdapter) bool {
	if len(got) != len(want) {
		return false
	}

	return slices.EqualFunc(sortStructuredMetadata(got), sortStructuredMetadata(want), func(a, b push.LabelAdapter) bool {
		return a.Name == b.Name && a.Value == b.Value
	})
}

func sortStructuredMetadata(labels push.LabelsAdapter) push.LabelsAdapter {
	cloned := slices.Clone(labels)
	slices.SortFunc(cloned, func(a, b push.LabelAdapter) int {
		if cmp := strings.Compare(a.Name, b.Name); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Value, b.Value)
	})
	return cloned
}
