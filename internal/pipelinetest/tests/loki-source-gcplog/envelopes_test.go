// Package lokisourcegcplog holds a utility for the loki-source-gcplog pipeline
// test. Each HTTP input in test.yaml is a GCP Pub/Sub push envelope whose
// message.data is a base64-encoded Cloud Logging LogEntry, so test.yaml can
// only hold the entries as blobs.
//
// logEntries below are the readable entries behind those blobs. To print the
// request bodies after changing them, run:
//
//	go test ./internal/pipelinetest/tests/loki-source-gcplog/ -run TestPushEnvelopes -v
//
// The test fails while test.yaml holds different bodies, so the two stay in
// sync.
package lokisourcegcplog

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/pipelinetest"
)

// pushSubscription is the subscription every envelope names.
const pushSubscription = "projects/test-project/subscriptions/test-push-sub"

// logEntries are the LogEntry values behind test.yaml's HTTP inputs, in order.
var logEntries = []string{
	`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"k8s_cluster","labels":{"cluster_name":"dev-us-central-42","location":"us-central1","project_id":"test-project"}},"timestamp":"2026-01-01T00:00:00Z","severity":"INFO","textPayload":"k8s_cluster message 0"}`,
	`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"gcs_bucket","labels":{"bucket_name":"loki-bucket","project_id":"test-project"}},"timestamp":"2026-01-01T00:00:01Z","severity":"INFO","textPayload":"gcs_bucket message 0"}`,
	`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"cloud_function","labels":{"function_name":"loki-fn","region":"us-central1"}},"timestamp":"2026-01-01T00:00:02Z","severity":"INFO","textPayload":"cloud_function message 0"}`,
	`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"k8s_cluster","labels":{"cluster_name":"tenant-cluster","location":"us-central1","project_id":"test-project"}},"timestamp":"2026-01-01T00:00:03Z","severity":"INFO","textPayload":"tenant k8s_cluster message 0"}`,
}

// pushEnvelope is the body GCP Pub/Sub POSTs to a push subscription endpoint.
type pushEnvelope struct {
	Message struct {
		Data      string `json:"data"`
		MessageID string `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

func TestPushEnvelopes(t *testing.T) {
	bb, err := os.ReadFile("test.yaml")
	require.NoError(t, err)
	var schema pipelinetest.TestSchema
	require.NoError(t, yaml.Unmarshal(bb, &schema))
	require.Len(t, schema.Inputs.HTTP, len(logEntries), "test.yaml needs one HTTP input per log entry")

	for i, entry := range logEntries {
		require.True(t, json.Valid([]byte(entry)), "log entry %d is not valid JSON", i)

		var envelope pushEnvelope
		envelope.Message.Data = base64.StdEncoding.EncodeToString([]byte(entry))
		envelope.Message.MessageID = strconv.Itoa(i)
		envelope.Subscription = pushSubscription
		body, err := json.Marshal(envelope)
		require.NoError(t, err)

		// Copy this body into the matching HTTP input in test.yaml.
		t.Logf("body %d:\n%s", i, body)
		require.Equal(t, string(body), schema.Inputs.HTTP[i].Body, "test.yaml body %d is out of date", i)
	}
}
