package loki

import (
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

func TestEntrySize(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry push.Entry
	}{
		{
			name:  "line only",
			entry: push.Entry{Timestamp: time.Unix(1700000000, 123456789), Line: "ok"},
		},
		{
			name: "structured metadata",
			entry: push.Entry{
				Timestamp:          time.Now(),
				Line:               "ok",
				StructuredMetadata: push.LabelsAdapter{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "abc123"}},
			},
		},
		{
			// Lengths of 128 bytes and more take more than one byte to encode.
			name:  "long line",
			entry: push.Entry{Timestamp: time.Now(), Line: strings.Repeat("x", 300)},
		},
		{
			name:  "zero timestamp and empty line",
			entry: push.Entry{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := Entry{Entry: tc.entry}
			encoded, err := tc.entry.Marshal()
			require.NoError(t, err)
			require.Equal(t, len(encoded), entry.Size())
		})
	}
}
