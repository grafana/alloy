package aws_firehose

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
)

// firehoseRequest implements AWS Firehose HTTP request format, according to the following appendix
// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#requestformat
type firehoseRequest struct {
	Timestamp int64            `json:"timestamp"`
	Records   []firehoseRecord `json:"records"`
}

// firehoseRecord is an envelope around a sole data record, received over Firehose HTTP API.
type firehoseRecord struct {
	Data string `json:"data"`
}

// cwLogsRecord is an envelope around a series of logging events, according to
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/SubscriptionFilters.html#DestinationKinesisExample
type cwLogsRecord struct {
	Owner               string       `json:"owner"`
	LogGroup            string       `json:"logGroup"`
	LogStream           string       `json:"logStream"`
	SubscriptionFilters []string     `json:"subscriptionFilters"`
	MessageType         string       `json:"messageType"`
	LogEvents           []cwLogEvent `json:"logEvents"`
}

// cwLogEvent is a single CloudWatch logging event.
type cwLogEvent struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

// firehoseResponse  is the expected response body.
// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#responseformat
type firehoseResponse struct {
	RequestID    string `json:"requestId"`
	Timestamp    int64  `json:"timestamp"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type recordOrigin string

const (
	originUnknown        recordOrigin = "unknown"
	originDirectPut      recordOrigin = "direct-put"
	originCloudwatchLogs recordOrigin = "cloudwatch-logs"
)

// decodeRecord base64-decodes Firehose record data and classifies the payload
// by format: plain decoded bytes are treated as Direct PUT records, while
// gzip-compressed decoded bytes are treated as CloudWatch Logs envelopes.
func decodeRecord(rec string) ([]byte, recordOrigin, error) {
	decodedRec, err := base64.StdEncoding.DecodeString(rec)
	if err != nil {
		return nil, originUnknown, errWithReason{
			err:    err,
			reason: "base64-decode",
		}
	}

	if !isGzipPayload(decodedRec) {
		return decodedRec, originDirectPut, nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(decodedRec))
	if err != nil {
		return nil, originCloudwatchLogs, fmt.Errorf("error creating gzip reader: %w", err)
	}
	defer reader.Close()

	var b bytes.Buffer
	if _, err := io.Copy(&b, reader); err != nil {
		return nil, originCloudwatchLogs, errWithReason{
			err:    err,
			reason: "gzip-deflate",
		}
	}

	return b.Bytes(), originCloudwatchLogs, nil
}

// gzipPrefix is used to check if bytes are gzip compressed.
// First two bytes are magic number and last is compression method, must be 8 (deflate).
// https://en.wikipedia.org/wiki/Gzip#File_structure
var gzipPrefix = []byte{0x1f, 0x8b, 8}

func isGzipPayload(data []byte) bool {
	return bytes.HasPrefix(data, gzipPrefix)
}
