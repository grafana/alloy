package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	r, err := Parse([]byte(`
topics:
  metrics: alloy-metrics
  logs: alloy-logs
  traces: alloy-traces
  profiles: alloy-profiles
partitions: 8
tenants:
  tenant-a: 0
  tenant-b: 1
  tenant-c: 5
retired: [3]
`))
	require.NoError(t, err)

	topic, ok := r.TopicFor(SignalLogs)
	require.True(t, ok)
	require.Equal(t, "alloy-logs", topic)
	require.Equal(t, []string{"alloy-logs", "alloy-metrics", "alloy-profiles", "alloy-traces"}, r.Topics())
	require.Equal(t, int32(8), r.Partitions())
	require.Equal(t, int32(5), r.MaxAssignedPartition())

	p, ok := r.PartitionFor("tenant-b")
	require.True(t, ok)
	require.Equal(t, int32(1), p)

	_, ok = r.PartitionFor("tenant-x")
	require.False(t, ok)

	tenant, ok := r.TenantFor(5)
	require.True(t, ok)
	require.Equal(t, "tenant-c", tenant)

	_, ok = r.TenantFor(3)
	require.False(t, ok, "retired partitions have no tenant")
	_, ok = r.TenantFor(2)
	require.False(t, ok)
}

func TestParse_NoTenants(t *testing.T) {
	r, err := Parse([]byte("topics: {metrics: t}\npartitions: 4\n"))
	require.NoError(t, err)
	require.Equal(t, int32(-1), r.MaxAssignedPartition())
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		err  string
	}{
		{"invalid yaml", "topics: [", "parsing tenant registry"},
		{"missing topics", "partitions: 4", "topics must map at least one signal"},
		{"unknown signal", "topics: {events: t}\npartitions: 4", `unknown signal "events"`},
		{"empty topic", "topics: {logs: ''}\npartitions: 4", `topic for signal "logs" must not be empty`},
		{"missing partitions", "topics: {logs: t}", "partitions must be greater than 0"},
		{"negative partition", "topics: {logs: t}\npartitions: 4\ntenants: {a: -1}", "out of range"},
		{"partition too large", "topics: {logs: t}\npartitions: 4\ntenants: {a: 4}", "out of range"},
		{"shared partition", "topics: {logs: t}\npartitions: 4\ntenants: {a: 1, b: 1}", "assigned to both"},
		{"retired partition", "topics: {logs: t}\npartitions: 4\ntenants: {a: 1}\nretired: [1]", "retired partition 1"},
		{"retired out of range", "topics: {logs: t}\npartitions: 4\nretired: [9]", "retired partition 9 out of range"},
		{"empty tenant", "topics: {logs: t}\npartitions: 4\ntenants: {'': 1}", "must not be empty"},
		{"duplicate tenant key", "topics: {logs: t}\npartitions: 4\ntenants:\n  a: 1\n  a: 2\n", "parsing tenant registry"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.in))
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestHolder(t *testing.T) {
	var h Holder
	require.Nil(t, h.Load())

	r, err := Parse([]byte("topics: {logs: t}\npartitions: 1\ntenants: {a: 0}"))
	require.NoError(t, err)
	h.Store(r)
	require.Same(t, r, h.Load())
}

func TestParse_SharedTopic(t *testing.T) {
	r, err := Parse([]byte("topics: {metrics: shared, logs: shared, traces: traces}\npartitions: 1"))
	require.NoError(t, err)
	require.Equal(t, []string{"shared", "traces"}, r.Topics())

	_, ok := r.TopicFor(SignalProfiles)
	require.False(t, ok, "unmapped signals have no topic")
}
