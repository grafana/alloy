package harness

import (
	"fmt"
	"reflect"

	"github.com/grafana/alloy/internal/component/common/loki"
)

type Assertion func(s snapshot) error

// LokiEntryMatch returns an Assertion that passes when the snapshot contains
// at least one Loki entry exactly matching want.
func LokiEntryMatch(want loki.Entry) Assertion {
	return func(s snapshot) error {
		for _, got := range s.loki {
			if equalEntry(got, want) {
				return nil
			}
		}
		return fmt.Errorf("entry not found: want=%#v got=%#v", want, s.loki)
	}
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

func equalEntry(got, want loki.Entry) bool {
	return reflect.DeepEqual(got.Labels, want.Labels) &&
		got.Timestamp.Equal(want.Timestamp) &&
		got.Line == want.Line &&
		reflect.DeepEqual(got.StructuredMetadata, want.StructuredMetadata)
}
