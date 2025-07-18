package database_observability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestBuildLokiEntry(t *testing.T) {
	entry := BuildLokiEntry(logging.LevelDebug, "test-operation", "test-instance", "This is a test log line")

	require.Len(t, entry.Labels, 3)
	require.Equal(t, database_observability.JobName, string(entry.Labels["job"]))
	require.Equal(t, "test-operation", string(entry.Labels["op"]))
	require.Equal(t, "test-instance", string(entry.Labels["instance"]))
	require.Equal(t, `level="debug" This is a test log line`, entry.Line)
}

func TestBuildLokiEntryWithTimestamp(t *testing.T) {
	entry := BuildLokiEntryWithTimestamp(logging.LevelInfo, "test-operation", "test-instance", "This is a test log line", 5)

	require.Equal(t, int64(5), entry.Entry.Timestamp.UnixNano())
	require.Equal(t, time.Unix(0, 5), entry.Entry.Timestamp)
}
