package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestEnrichWithFileDiscovery(t *testing.T) {
	// Wait for services to be ready
	time.Sleep(2 * time.Second)

	// Send test logs directly to API
	sendTestLogs(t)

	// Query Loki for enriched logs
	var logResponse common.LogResponse
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := common.FetchDataFromURL(
			"http://127.0.0.1:3100/loki/api/v1/query?query=%7Btest_name%3D%22network_device_enriched%22%7D",
			&logResponse,
		)
		assert.NoError(c, err)
		if len(logResponse.Data.Result) == 0 {
			return
		}

		// Verify we got all logs
		result := logResponse.Data.Result[0]
		assert.Equal(c, 3, len(result.Values), "should have 3 log entries")

		// Verify labels were enriched
		expectedLabels := map[string]string{
			"environment": "production",
			"datacenter":  "us-east",
			"role":        "core-router",
			"host":        "router1.example.com",
		}
		for k, v := range expectedLabels {
			assert.Equal(c, v, result.Stream[k], "label %s should be %s", k, v)
		}
	}, 10*time.Second, 500*time.Millisecond)
}

func sendTestLogs(t *testing.T) {
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

	pushReq := common.LogResponse{
		Data: struct {
			ResultType string           `json:"resultType"`
			Result     []common.LogData `json:"result"`
		}{
			Result: []common.LogData{{
				Stream: map[string]string{
					"host": "router1.example.com",
				},
				Values: values,
			}}},
	}

	body, err := json.Marshal(pushReq)
	require.NoError(t, err)

	resp, err := http.Post("http://127.0.0.1:1514/loki/api/v1/push", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}
