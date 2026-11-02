package deps

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckLeaders(t *testing.T) {
	// Captured from `kafka-topics.sh --describe` on apache/kafka:4.1.0.
	const header = "Topic: t1\tTopicId: OLwXCPjOSuKzO2x_6LD4_w\tPartitionCount: 2\tReplicationFactor: 1\tConfigs: min.insync.replicas=1\n"
	partition := func(id, leader string) string {
		return "\tTopic: t1\tPartition: " + id + "\tLeader: " + leader + "\tReplicas: 1\tIsr: 1\tElr: \tLastKnownElr: \n"
	}

	tests := []struct {
		name     string
		describe string
		wantErr  bool
	}{
		{"all partitions led", header + partition("0", "1") + partition("1", "1"), false},
		{"no leader yet", header + partition("0", "1") + partition("1", "none"), true},
		{"negative leader", header + partition("0", "1") + partition("1", "-1"), true},
		{"partitions missing", header + partition("0", "1"), true},
		{"empty output", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkLeaders(tt.describe, 2)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
