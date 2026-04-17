package harness

import (
	"fmt"
	"reflect"
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

type AssertionErrors []error

func (e AssertionErrors) Error() string {
	var builder strings.Builder
	for _, err := range e {
		builder.WriteString("- ")
		builder.WriteString(err.Error())
		builder.WriteByte('\n')
	}
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
		text: fmt.Sprintf("line=%q", line),
	}
}

// LokiEntryLabels returns an EntryMatcher that matches the Loki entry labels
// exactly.
func LokiEntryLabels(labels model.LabelSet) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			return reflect.DeepEqual(entry.Labels, labels)
		},
		text: "labels=" + labels.String(),
	}
}

// LokiEntryStructuredMetadata returns an EntryMatcher that matches the Loki
// entry structured metadata exactly.
func LokiEntryStructuredMetadata(metadata push.LabelsAdapter) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			return reflect.DeepEqual(entry.StructuredMetadata, metadata)
		},
		text: fmt.Sprintf("structured_metadata=%v", metadata),
	}
}

// LokiEntryTimestamp returns an EntryMatcher that matches the Loki entry
// timestamp exactly.
func LokiEntryTimestamp(ts time.Time) EntryMatcher {
	return EntryMatcher{
		match: func(entry loki.Entry) bool {
			return entry.Timestamp.Equal(ts)
		},
		text: "timestamp=" + ts.Format(time.RFC3339Nano),
	}
}
