//go:build alloyintegrationtests

package main

import (
	"bytes"
	"compress/gzip"
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
	// Send Direct PUT records: 2 ARNs x 3 batches x 10 records = 60 entries.
	err := pushFirehoseDirectPutRecords(3, 10,
		"arn:aws:firehose:us-east-1:123456789:deliverystream/stream-1",
		"arn:aws:firehose:us-east-1:123456789:deliverystream/stream-2",
	)
	require.NoError(t, err)

	// Send CloudWatch Logs records: 3 batches x 1 record x 10 events = 30 entries.
	err = pushFirehoseCWLogsRecords(3, 10,
		"arn:aws:firehose:us-east-1:123456789:deliverystream/cw-stream",
	)
	require.NoError(t, err)

	common.AssertLogsPresent(
		t,
		90,
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
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "arn:aws:firehose:us-east-1:123456789:deliverystream/cw-stream",
			},
			StructuredMetadata: map[string]string{
				"log_group": "/aws/lambda/test-function",
				"msg_type":  "DATA_MESSAGE",
			},
			EntryCount: 30,
		},
	)

	common.AssertLabelsNotIndexed(t, "source_arn", "log_group", "msg_type")
}

// firehose request/record types matching the AWS Firehose HTTP delivery format.

type firehoseRequest struct {
	RequestID string           `json:"requestId"`
	Timestamp int64            `json:"timestamp"`
	Records   []firehoseRecord `json:"records"`
}

type firehoseRecord struct {
	Data string `json:"data"`
}

type cwLogsRecord struct {
	Owner               string       `json:"owner"`
	LogGroup            string       `json:"logGroup"`
	LogStream           string       `json:"logStream"`
	SubscriptionFilters []string     `json:"subscriptionFilters"`
	MessageType         string       `json:"messageType"`
	LogEvents           []cwLogEvent `json:"logEvents"`
}

type cwLogEvent struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

// pushFirehoseDirectPutRecords sends Direct PUT Firehose requests to the
// running Alloy component. It sends `batches` requests per source ARN, each
// containing `recordsPerBatch` base64-encoded plain text records.
func pushFirehoseDirectPutRecords(batches, recordsPerBatch int, sourceARNs ...string) error {
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

			if err := sendFirehoseRequest(arn, i, now, records); err != nil {
				return err
			}
		}
	}

	return nil
}

// pushFirehoseCWLogsRecords sends CloudWatch Logs formatted Firehose requests.
// Each batch contains one gzip-compressed CW Logs record with `eventsPerBatch`
// log events.
func pushFirehoseCWLogsRecords(batches, eventsPerBatch int, sourceARNs ...string) error {
	now := time.Now().UTC()

	for _, arn := range sourceARNs {
		for i := range batches {
			events := make([]cwLogEvent, 0, eventsPerBatch)
			for j := range eventsPerBatch {
				events = append(events, cwLogEvent{
					ID:        fmt.Sprintf("event-%d-%d", i, j),
					Timestamp: now.UnixMilli(),
					Message:   fmt.Sprintf("cw log event %d-%d from %s", i, j, arn),
				})
				now = now.Add(time.Second)
			}

			encoded, err := buildCWLogsRecord(cwLogsRecord{
				Owner:               "123456789012",
				LogGroup:            "/aws/lambda/test-function",
				LogStream:           fmt.Sprintf("2024/01/01/[$LATEST]stream%d", i),
				SubscriptionFilters: []string{"test-filter"},
				MessageType:         "DATA_MESSAGE",
				LogEvents:           events,
			})
			if err != nil {
				return fmt.Errorf("build CW Logs record: %w", err)
			}

			records := []firehoseRecord{{Data: encoded}}
			if err := sendFirehoseRequest(arn, i, now, records); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildCWLogsRecord marshals a CloudWatch Logs record to JSON, gzip-compresses
// it, and base64-encodes the result — matching the format Firehose uses for
// CloudWatch Logs subscription data.
func buildCWLogsRecord(rec cwLogsRecord) (string, error) {
	jsonData, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("marshal CW Logs record: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		return "", fmt.Errorf("gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("gzip close: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func sendFirehoseRequest(sourceARN string, batchIndex int, ts time.Time, records []firehoseRecord) error {
	requestID := fmt.Sprintf("test-request-%d", batchIndex)
	reqBody := firehoseRequest{
		RequestID: requestID,
		Timestamp: ts.UnixMilli(),
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
	req.Header.Set("X-Amz-Firehose-Source-Arn", sourceARN)
	req.Header.Set("X-Amz-Firehose-Protocol-Version", "1.0")
	req.Header.Set("User-Agent", "Amazon Kinesis Data Firehose Agent/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send firehose request: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d for ARN %s batch %d",
			resp.StatusCode, sourceARN, batchIndex)
	}

	return nil
}
