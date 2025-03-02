package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

type PushRequest struct {
	Streams []Stream `json:"streams"`
}

type Stream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

func TestEnricherWithFileDiscovery(t *testing.T) {
	// Wait for services to be ready
	time.Sleep(2 * time.Second)

	// Generate and send test logs
	go generateLogs(t)

	// Query Loki for enriched logs
	var logResponse common.LogResponse
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := common.FetchDataFromURL(
			"http://localhost:3100/loki/api/v1/query?query={test_name=\"network_device_enriched\"}",
			&logResponse,
		)
		assert.NoError(c, err)
		assert.NotEmpty(c, logResponse.Data.Result)
	}, 10*time.Second, 500*time.Millisecond)

	// Verify the enriched logs have the expected labels
	for _, result := range logResponse.Data.Result {
		labels := result.Stream
		// Check that metadata was added
		require.Contains(t, labels, "environment")
		require.Contains(t, labels, "datacenter")
		require.Contains(t, labels, "role")

		// Verify expected values
		require.Equal(t, "production", labels["environment"])
		require.Equal(t, "us-east", labels["datacenter"])
		require.Equal(t, "core-router", labels["role"])
	}
}

func generateLogs(t *testing.T) {
	networkLogs := []string{
		"%LINK-3-UPDOWN: Interface GigabitEthernet1/0/1, changed state to up",
		"%SEC-6-IPACCESSLOGP: list 102 denied tcp 10.1.1.1(1234) -> 10.1.1.2(80), 1 packet",
		"%SYS-5-CONFIG_I: Configured from console by admin on vty0 (10.1.1.1)",
	}

	stream := Stream{
		Stream: map[string]string{
			"host": "router1.example.com",
			"job":  "network_device_logs",
		},
		Values: make([][]string, 0),
	}

	// Add timestamps and messages
	now := time.Now()
	for _, msg := range networkLogs {
		stream.Values = append(stream.Values, []string{
			now.Format(time.RFC3339Nano),
			msg,
		})
		now = now.Add(time.Second)
	}

	pushReq := PushRequest{
		Streams: []Stream{stream},
	}

	// Send logs every few seconds
	for {
		body, err := json.Marshal(pushReq)
		require.NoError(t, err)

		resp, err := http.Post("http://127.0.0.1:1514/loki/api/v1/push", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		resp.Body.Close()

		time.Sleep(5 * time.Second)
	}
}
