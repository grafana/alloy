//go:build alloyintegrationtests && !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestEnrichWithFileDiscovery(t *testing.T) {
	// Send test logs directly to API
	sendTestLogsForDevice(t, "router1.example.com")

	// Verify logs were enriched with expected labels
	common.AssertLogsPresent(t, common.ExpectedLogResult{
		Labels: map[string]string{
			"environment": "production",
			"datacenter":  "us-east",
			"role":        "core-router",
			"rack":        "rack1",
			"host":        "router1.example.com",
			"job":         "network_device_logs",
		},
		EntryCount: 3,
	})
}

func TestEnrichWithMissingLabels(t *testing.T) {
	// Send test logs for unknown device
	sendTestLogsForDevice(t, "unknown.example.com")

	// Verify logs passed through without enrichment
	common.AssertLogsPresent(t, common.ExpectedLogResult{
		Labels: map[string]string{
			"host": "unknown.example.com",
		},
		EntryCount: 3,
	})
}

func sendTestLogsForDevice(t *testing.T, hostname string) {
	networkLogs := []string{
		"%LINK-3-UPDOWN: Interface GigabitEthernet1/0/1, changed state to up",
		"%SEC-6-IPACCESSLOGP: list 102 denied tcp 10.1.1.1(1234) -> 10.1.1.2(80), 1 packet",
		"%SYS-5-CONFIG_I: Configured from console by admin on vty0 (10.1.1.1)",
	}

	now := time.Now()
	values := make([][2]string, 0, len(networkLogs))
	for _, msg := range networkLogs {
		values = append(values, [2]string{
			fmt.Sprintf("%d", now.UnixNano()),
			msg,
		})
		now = now.Add(time.Second)
	}

	pushReq := common.PushRequest{
		Streams: []common.LogData{{
			Stream: map[string]string{
				"host":      hostname,
				"test_name": common.SanitizeTestName(t),
			},
			Values: values,
		}},
	}

	body, err := json.Marshal(pushReq)
	require.NoError(t, err)

	resp, err := http.Post("http://localhost:1514/loki/api/v1/push", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}
