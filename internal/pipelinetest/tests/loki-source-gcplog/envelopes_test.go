package lokisourcegcplog

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// Prints the GCP Pub/Sub push bodies for test.yaml:
//
//	go test ./internal/pipelinetest/tests/loki-source-gcplog/ -run TestPushEnvelopes -v
func TestPushEnvelopes(t *testing.T) {
	logEntries := []string{
		`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"k8s_cluster","labels":{"cluster_name":"dev-us-central-42","location":"us-central1","project_id":"test-project"}},"timestamp":"2026-01-01T00:00:00Z","severity":"INFO","textPayload":"k8s_cluster message 0"}`,
		`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"gcs_bucket","labels":{"bucket_name":"loki-bucket","project_id":"test-project"}},"timestamp":"2026-01-01T00:00:01Z","severity":"INFO","textPayload":"gcs_bucket message 0"}`,
		`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"cloud_function","labels":{"function_name":"loki-fn","region":"us-central1"}},"timestamp":"2026-01-01T00:00:02Z","severity":"INFO","textPayload":"cloud_function message 0"}`,
		`{"logName":"projects/test-project/logs/integration-test","resource":{"type":"k8s_cluster","labels":{"cluster_name":"tenant-cluster","location":"us-central1","project_id":"test-project"}},"timestamp":"2026-01-01T00:00:03Z","severity":"INFO","textPayload":"tenant k8s_cluster message 0"}`,
	}
	for i, entry := range logEntries {
		body, err := json.Marshal(map[string]any{
			"message": map[string]string{
				"data":      base64.StdEncoding.EncodeToString([]byte(entry)),
				"messageId": strconv.Itoa(i),
			},
			"subscription": "projects/test-project/subscriptions/test-push-sub",
		})
		require.NoError(t, err)
		t.Logf("body %d:\n%s", i, body)
	}
}
