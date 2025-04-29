package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestBuildLokiEntry(t *testing.T) {
	t.Run("without manual timestamp", func(t *testing.T) {
		entry := buildLokiEntry(logging.LevelDebug, "test-operation", "test-instance", "This is a test log line", nil)

		require.Len(t, entry.Labels, 3)
		require.Equal(t, database_observability.JobName, string(entry.Labels["job"]))
		require.Equal(t, "test-operation", string(entry.Labels["op"]))
		require.Equal(t, "test-instance", string(entry.Labels["instance"]))
		require.Equal(t, `level="debug" This is a test log line`, entry.Line)
	})

	t.Run("with manual timestamp", func(t *testing.T) {
		ts := float64(5)
		entry := buildLokiEntry(logging.LevelInfo, "test-operation", "test-instance", "This is a test log line", &ts)

		require.Equal(t, int64(ts), entry.Entry.Timestamp.UnixNano())
		require.Equal(t, time.Unix(0, int64(ts)), entry.Entry.Timestamp)
	})
}
