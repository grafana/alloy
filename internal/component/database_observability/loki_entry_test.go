package database_observability

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/loki/pkg/push"
)

func TestBuildLokiEntry(t *testing.T) {
	entry := BuildLokiEntry(logging.LevelDebug, "test-operation", "This is a test log line")

	require.Len(t, entry.Labels, 1)
	require.Equal(t, "test-operation", string(entry.Labels["op"]))
	require.Equal(t, `level="debug" This is a test log line`, entry.Line)
}

func TestBuildLokiEntryWithTimestamp(t *testing.T) {
	entry := BuildLokiEntryWithTimestamp(logging.LevelInfo, "test-operation", "This is a test log line", 5)

	require.Equal(t, int64(5), entry.Entry.Timestamp.UnixNano())
	require.Equal(t, time.Unix(0, 5), entry.Entry.Timestamp)
}

func TestBuildLokiEntryWithStructuredMetadata(t *testing.T) {
	e := BuildLokiEntryWithStructuredMetadata(
		logging.LevelInfo,
		"op_test",
		`key="value"`,
		push.LabelsAdapter{
			push.LabelAdapter{Name: "query_fingerprint", Value: "abc123"},
		},
	)
	require.Equal(t, model.LabelValue("op_test"), e.Labels["op"])
	require.Equal(t, `level="info" key="value"`, e.Line)
	require.Len(t, e.StructuredMetadata, 1)
	require.Equal(t, "query_fingerprint", e.StructuredMetadata[0].Name)
	require.Equal(t, "abc123", e.StructuredMetadata[0].Value)
}

func TestBuildLokiEntryWithTimestampAndStructuredMetadata(t *testing.T) {
	e := BuildLokiEntryWithTimestampAndStructuredMetadata(
		logging.LevelInfo,
		"op_test",
		`key="value"`,
		42,
		push.LabelsAdapter{
			push.LabelAdapter{Name: "query_fingerprint", Value: "abc123"},
		},
	)
	require.Equal(t, model.LabelValue("op_test"), e.Labels["op"])
	require.Equal(t, `level="info" key="value"`, e.Line)
	require.Equal(t, int64(42), e.Entry.Timestamp.UnixNano())
	require.Len(t, e.StructuredMetadata, 1)
	require.Equal(t, "abc123", e.StructuredMetadata[0].Value)
}
