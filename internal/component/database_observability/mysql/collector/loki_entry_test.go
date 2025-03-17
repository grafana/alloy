package collector

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestBuildLokiEntry(t *testing.T) {
	entry := buildLokiEntry("test-operation", "test-instance", "This is a test log line")

	require.Len(t, entry.Labels, 3)
	require.Equal(t, database_observability.JobName, string(entry.Labels["job"]))
	require.Equal(t, "test-operation", string(entry.Labels["op"]))
	require.Equal(t, "test-instance", string(entry.Labels["instance"]))
	require.Equal(t, "This is a test log line", entry.Line)
}
