package loki

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

func TestEntrySize(t *testing.T) {
	metadata := push.LabelsAdapter{{Name: "foo", Value: "bar"}}
	entry := Entry{Entry: push.Entry{
		Timestamp:          time.Now(),
		Line:               "ok",
		StructuredMetadata: metadata,
	}}
	expected := len(entry.Line) + 12
	for _, label := range metadata {
		expected += label.Size()
	}
	require.Equal(t, expected, entry.Size())
}
