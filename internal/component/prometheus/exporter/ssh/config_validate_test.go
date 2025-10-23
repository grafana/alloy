package ssh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTargetValidateNotAHost ensures invalid hostnames are rejected.
func TestTargetValidateNotAHost(t *testing.T) {
	tgt := Target{
		Address:        "not a host",
		Port:           22,
		Username:       "user",
		CommandTimeout: 5 * time.Second,
	}
	err := tgt.Validate()
	require.Error(t, err)
}

// TestCustomMetricValidateUnsafeCommand ensures dangerous commands are rejected.
func TestCustomMetricValidateUnsafeCommand(t *testing.T) {
	cm := CustomMetric{
		Name:    "m1",
		Command: "uname -a `rm -rf /`",
		Type:    "gauge",
	}
	err := cm.Validate()
	require.Error(t, err)
}
