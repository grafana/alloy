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

// LokiEntries returns an Assertion over Loki entries matched by all provided
// matchers. When want is nil, at least one matching entry must exist. When
// want is non-nil, exactly *want matching entries must exist.
func LokiEntries(want *int, matchers ...EntryMatcher) Assertion {
	return func(s snapshot) error {
		var got int
		for _, entry := range s.loki {
			if entryMatches(entry, matchers...) {
				got++
			}
		}

		conditions := make([]string, 0, len(matchers))
		for _, matcher := range matchers {
			if matcher.text == "" {
				continue
			}
			conditions = append(conditions, matcher.text)
		}

		if want != nil {
			if got == *want {
				return nil
			}

			message := fmt.Sprintf("want %d, got %d", *want, got)
			if len(conditions) > 0 {
				message += " for " + strings.Join(conditions, ", ")
			}

			return AssertionError{
				Kind:    "loki.entry",
				Message: message,
			}
		}

		if got > 0 {
			return nil
		}

		message := "no matching entry found"
		if len(conditions) > 0 {
			message += " for " + strings.Join(conditions, ", ")
		}

		return AssertionError{
			Kind:    "loki.entry",
			Message: message,
		}
	}
}

func entryMatches(entry loki.Entry, matchers ...EntryMatcher) bool {
	for _, matcher := range matchers {
		if !matcher.match(entry) {
			return false
		}
	}
	return true
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

// LokiEntryLabels returns an EntryMatcher for Loki entry labels. When partial
// is false, labels must match exactly. When partial is true, the entry labels
// must contain at least the provided labels.
func LokiEntryLabels(labels model.LabelSet, partial bool) EntryMatcher {
	var match func(entry loki.Entry) bool
	if partial {
		match = func(entry loki.Entry) bool {
			return labelSetContains(entry.Labels, labels)
		}
	} else {
		match = func(entry loki.Entry) bool {
			return reflect.DeepEqual(entry.Labels, labels)
		}
	}

	return EntryMatcher{
		match: match,
		text:  renderLabelSet(labels),
	}
}

// LokiEntryStructuredMetadata returns an EntryMatcher for Loki entry
// structured metadata. When partial is false, structured metadata must match
// exactly. When partial is true, the entry metadata must contain at least the
// provided labels.
func LokiEntryStructuredMetadata(metadata push.LabelsAdapter, partial bool) EntryMatcher {
	var match func(entry loki.Entry) bool
	if partial {
		match = func(entry loki.Entry) bool {
			return structuredMetadataContains(entry.StructuredMetadata, metadata)
		}

	} else {

		match = func(entry loki.Entry) bool {
			return structuredMetadataEqual(entry.StructuredMetadata, metadata)
		}
	}

	return EntryMatcher{
		match: match,
		text:  renderStructuredMetadata(metadata),
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

func labelSetContains(got, want model.LabelSet) bool {
	for name, value := range want {
		if got[name] != value {
			return false
		}
	}
	return true
}

func structuredMetadataContains(got, want push.LabelsAdapter) bool {
	if len(want) == 0 {
		return true
	}

	gotMap := make(map[string]string, len(got))
	for _, label := range got {
		gotMap[label.Name] = label.Value
	}

	for _, label := range want {
		if gotMap[label.Name] != label.Value {
			return false
		}
	}
	return true
}
