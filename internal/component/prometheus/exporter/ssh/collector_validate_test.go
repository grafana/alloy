package ssh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTargetValidate_InvalidAddress(t *testing.T) {
	// Invalid hostname should be rejected
	tgt := Target{
		Address:        "http://example.com", // scheme not allowed
		Port:           22,
		Username:       "user",
		CommandTimeout: 5 * time.Second,
	}
	err := tgt.Validate()
	require.Error(t, err)
}

func TestCustomMetricValidate_UnsafeCommand(t *testing.T) {
	// Commands containing dangerous characters should be rejected
	cm := CustomMetric{
		Name:    "m1",
		Command: "echo 1; rm -rf /",
		Type:    "gauge",
	}
	err := cm.Validate()
	require.Error(t, err)
}
