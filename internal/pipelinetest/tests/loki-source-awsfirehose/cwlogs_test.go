// Package lokisourceawsfirehose holds a helper for the loki-source-awsfirehose
// pipeline test. Firehose wraps CloudWatch Logs subscription data as a
// gzip-compressed, base64-encoded JSON record, which test.yaml can only hold as
// an opaque blob. This file keeps the readable record, generates the blob from
// it, and checks that the blob in test.yaml still decodes to it.
//
// To print a fresh blob after changing cwLogsRecord, run:
//
//	go test ./internal/pipelinetest/tests/loki-source-awsfirehose/ -run TestCWLogsRecordBlob -v
package lokisourceawsfirehose

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// cwLogsRecord is the CloudWatch Logs record carried by the CloudWatch input in
// test.yaml.
var cwLogsRecord = map[string]any{
	"owner":               "123456789012",
	"logGroup":            "/aws/lambda/test-function",
	"logStream":           "2026/01/01/[$LATEST]stream0",
	"subscriptionFilters": []any{"test-filter"},
	"messageType":         "DATA_MESSAGE",
	"logEvents": []any{
		map[string]any{"id": "event-0", "timestamp": 1767225604000, "message": "cw log event 0 from cw-stream"},
		map[string]any{"id": "event-1", "timestamp": 1767225605000, "message": "cw log event 1 from cw-stream"},
	},
}

// cwLogsSourceARN identifies the CloudWatch input among the HTTP inputs.
const cwLogsSourceARN = "arn:aws:firehose:us-east-1:123456789012:deliverystream/cw-stream"

func encodeCWLogsRecord(t *testing.T, rec any) string {
	t.Helper()
	raw, err := json.Marshal(rec)
	require.NoError(t, err)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err = gz.Write(raw)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func decodeCWLogsRecord(t *testing.T, blob string) any {
	t.Helper()
	compressed, err := base64.StdEncoding.DecodeString(blob)
	require.NoError(t, err)
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	raw, err := io.ReadAll(gz)
	require.NoError(t, err)

	var rec any
	require.NoError(t, json.Unmarshal(raw, &rec))
	return rec
}

// TestCWLogsRecordBlob checks that the blob in test.yaml decodes to
// cwLogsRecord. It compares decoded values rather than blobs, because gzip
// output can differ between Go versions.
func TestCWLogsRecordBlob(t *testing.T) {
	want := decodeCWLogsRecord(t, encodeCWLogsRecord(t, cwLogsRecord))
	t.Logf("blob for cwLogsRecord:\n%s", encodeCWLogsRecord(t, cwLogsRecord))

	bb, err := os.ReadFile("test.yaml")
	require.NoError(t, err)

	var schema struct {
		Inputs struct {
			HTTP []struct {
				Headers map[string]string `yaml:"headers"`
				Body    string            `yaml:"body"`
			} `yaml:"http"`
		} `yaml:"inputs"`
	}
	require.NoError(t, yaml.Unmarshal(bb, &schema))

	var found bool
	for _, input := range schema.Inputs.HTTP {
		if input.Headers["X-Amz-Firehose-Source-Arn"] != cwLogsSourceARN {
			continue
		}
		found = true

		var body struct {
			Records []struct {
				Data string `json:"data"`
			} `json:"records"`
		}
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(input.Body)), &body))
		require.Len(t, body.Records, 1)
		require.Equal(t, want, decodeCWLogsRecord(t, body.Records[0].Data),
			"the CloudWatch blob in test.yaml does not match cwLogsRecord; replace it with the blob logged by this test")
	}
	require.True(t, found, "no HTTP input in test.yaml has source ARN %s", cwLogsSourceARN)
}
