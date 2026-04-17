//go:build alloyintegrationtests

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const firehoseURL = "http://127.0.0.1:1517/awsfirehose/api/v1/push"

func TestLokiFirehose(t *testing.T) {
	err := pushFirehoseRecords(3, 10,
		"arn:aws:firehose:us-east-1:123456789:deliverystream/stream-1",
		"arn:aws:firehose:us-east-1:123456789:deliverystream/stream-2",
	)
	require.NoError(t, err)

	common.AssertLogsPresent(
		t,
		60,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "arn:aws:firehose:us-east-1:123456789:deliverystream/stream-1",
			},
			EntryCount: 30,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "arn:aws:firehose:us-east-1:123456789:deliverystream/stream-2",
			},
			EntryCount: 30,
		},
	)

	common.AssertLabelsNotIndexed(t, "source_arn")
}

type firehoseRequest struct {
	RequestID string           `json:"requestId"`
	Timestamp int64            `json:"timestamp"`
	Records   []firehoseRecord `json:"records"`
}

type firehoseRecord struct {
	Data string `json:"data"`
}

// pushFirehoseRecords sends Direct PUT Firehose requests to the running Alloy
// component. It sends `batches` requests per source ARN, each containing
// `recordsPerBatch` base64-encoded plain text records.
func pushFirehoseRecords(batches, recordsPerBatch int, sourceARNs ...string) error {
	now := time.Now().UTC()

	for _, arn := range sourceARNs {
		for i := range batches {
			records := make([]firehoseRecord, 0, recordsPerBatch)
			for j := range recordsPerBatch {
				line := fmt.Sprintf("request %d log line %d from %s at %s",
					i, j, arn, now.Format(time.RFC3339Nano))
				records = append(records, firehoseRecord{
					Data: base64.StdEncoding.EncodeToString([]byte(line)),
				})
				now = now.Add(time.Second)
			}

			requestID := fmt.Sprintf("test-request-%d", i)
			reqBody := firehoseRequest{
				RequestID: requestID,
				Timestamp: now.UnixMilli(),
				Records:   records,
			}

			body, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("marshal firehose request: %w", err)
			}

			req, err := http.NewRequest(http.MethodPost, firehoseURL, bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("create HTTP request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Amz-Firehose-Request-Id", requestID)
			req.Header.Set("X-Amz-Firehose-Source-Arn", arn)
			req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
			req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("send firehose request: %w", err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code %d for ARN %s batch %d",
					resp.StatusCode, arn, i)
			}
		}
	}

	return nil
}
