package ssh_exporter

import (
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestTargetValidate_InvalidAddress(t *testing.T) {
    tgt := Target{
        Address:        "not a host",
        Port:           22,
        Username:       "user",
        CommandTimeout: 5 * time.Second,
    }
    err := tgt.Validate()
    require.Error(t, err)
}

func TestCustomMetricValidate_UnsafeCommand(t *testing.T) {
    cm := CustomMetric{
        Name:    "m1",
        Command: "uname -a `rm -rf /`",
        Type:    "gauge",
    }
    err := cm.Validate()
    require.Error(t, err)
}