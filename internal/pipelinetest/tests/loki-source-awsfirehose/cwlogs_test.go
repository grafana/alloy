// Package lokisourceawsfirehose holds a utility for the loki-source-awsfirehose
// pipeline test. Firehose carries CloudWatch Logs data as gzip-compressed,
// base64-encoded JSON, so test.yaml can only hold it as a blob. The component
// detects CloudWatch Logs by the gzip header, so plain base64 is not enough.
//
// To print the blob for cwLogsRecord, run:
//
//	go test ./internal/pipelinetest/tests/loki-source-awsfirehose/ -run TestCWLogsRecordBlob -v
package lokisourceawsfirehose

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// cwLogsRecord is the CloudWatch Logs record behind the blob in test.yaml.
const cwLogsRecord = `{"owner":"123456789012","logGroup":"/aws/lambda/test-function","logStream":"2026/01/01/[$LATEST]stream0","subscriptionFilters":["test-filter"],"messageType":"DATA_MESSAGE","logEvents":[{"id":"event-0","timestamp":1767225604000,"message":"cw log event 0 from cw-stream"},{"id":"event-1","timestamp":1767225605000,"message":"cw log event 1 from cw-stream"}]}`

func TestCWLogsRecordBlob(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := io.WriteString(gz, cwLogsRecord)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	blob := base64.StdEncoding.EncodeToString(buf.Bytes())
	t.Logf("blob:\n%s", blob)

	compressed, err := base64.StdEncoding.DecodeString(blob)
	require.NoError(t, err)
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	decoded, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, cwLogsRecord, string(decoded))
}
