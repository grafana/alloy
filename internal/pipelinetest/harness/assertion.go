package harness

import (
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

type Assertion func(s snapshot) error

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
			return fmt.Errorf("unexpected entry count: want=%d got=%d", want, len(s.loki))
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
				if !matcher(entry) {
					matched = false
					break
				}
			}
			if matched {
				return nil
			}
		}
		return fmt.Errorf("matching entry not found: got=%#v", s.loki)
	}
}

type EntryMatcher func(entry loki.Entry) bool

// LokiEntryLine returns an EntryMatcher that matches the Loki entry line
// exactly.
func LokiEntryLine(line string) EntryMatcher {
	return func(entry loki.Entry) bool {
		return entry.Line == line
	}
}

// LokiEntryLabels returns an EntryMatcher that matches the Loki entry labels
// exactly.
func LokiEntryLabels(labels model.LabelSet) EntryMatcher {
	return func(entry loki.Entry) bool {
		return reflect.DeepEqual(entry.Labels, labels)
	}
}

// LokiEntryStructuredMetadata returns an EntryMatcher that matches the Loki
// entry structured metadata exactly.
func LokiEntryStructuredMetadata(metadata push.LabelsAdapter) EntryMatcher {
	return func(entry loki.Entry) bool {
		return reflect.DeepEqual(entry.StructuredMetadata, metadata)
	}
}

// LokiEntryTimestamp returns an EntryMatcher that matches the Loki entry
// timestamp exactly.
func LokiEntryTimestamp(ts time.Time) EntryMatcher {
	return func(entry loki.Entry) bool {
		return entry.Timestamp.Equal(ts)
	}
}
