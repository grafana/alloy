package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestEnrichWithFileDiscovery(t *testing.T) {
	// Send test logs directly to API
	sendTestLogsForDevice(t, "router1.example.com")

	// Verify logs were enriched with expected labels
	common.AssertLogsPresent(t, "network_device_enriched", map[string]string{
		"environment": "production",
		"datacenter":  "us-east",
		"role":        "core-router",
		"rack":        "rack1",
		"host":        "router1.example.com",
	}, 3)
	common.AssertLogsMissing(t, "network_device_enriched",
		"__meta_rack",
	)
}

func TestEnrichWithMissingLabels(t *testing.T) {
	// Send test logs for unknown device
	sendTestLogsForDevice(t, "unknown.example.com")

	// Verify logs passed through without enrichment
	common.AssertLogsPresent(t, "network_device_enriched", map[string]string{
		"host": "unknown.example.com",
	}, 3)
	common.AssertLogsMissing(t, "network_device_enriched",
		"environment",
		"datacenter",
		"role",
		"rack",
	)
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
				"host": hostname,
			},
			Values: values,
		}},
	}

	body, err := json.Marshal(pushReq)
	require.NoError(t, err)

	resp, err := http.Post("http://127.0.0.1:1514/loki/api/v1/push", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}
