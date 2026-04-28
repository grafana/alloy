//go:build alloyintegrationtests

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

const lokiAPIURL = "http://localhost:1515/loki/api/v1/push"

func TestLokiAPI(t *testing.T) {
	require.NoError(t, pushJSON("frontend", "backend"))
	require.NoError(t, pushProto("frontend-proto", "backend-proto"))

	common.AssertLogsPresent(
		t,
		120,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "frontend",
			},
			StructuredMetadata: map[string]string{
				"content_type": "json",
			},
			EntryCount: 30,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "backend",
			},
			StructuredMetadata: map[string]string{
				"content_type": "json",
			},
			EntryCount: 30,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "frontend-proto",
			},
			StructuredMetadata: map[string]string{
				"content_type": "protobuf",
			},
			EntryCount: 30,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "backend-proto",
			},
			StructuredMetadata: map[string]string{
				"content_type": "protobuf",
			},
			EntryCount: 30,
		},
	)

	common.AssertLabelsNotIndexed(t, "app")
}

func pushJSON(apps ...string) error {
	var (
		now     = time.Now()
		streams = make([]common.LogData, 0, len(apps))
	)

	for _, app := range apps {
		values := make([]common.LogEntry, 0, 30)
		for i := range 30 {
			values = append(values, common.LogEntry{
				Timestamp: fmt.Sprintf("%d", now.UnixNano()),
				Line:      fmt.Sprintf("log line %d from %s", i, app),
			})
			now = now.Add(time.Second)
		}

		streams = append(streams, common.LogData{
			Stream: map[string]string{
				"app":          app,
				"content_type": "json",
			},
			Values: values,
		})
	}

	body, err := json.Marshal(common.PushRequest{Streams: streams})
	if err != nil {
		return err
	}

	resp, err := http.Post(lokiAPIURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func pushProto(apps ...string) error {
	var (
		pr  push.PushRequest
		now = time.Now()
	)

	for _, app := range apps {
		entries := make([]push.Entry, 0, 30)
		for i := range 30 {
			entries = append(entries, push.Entry{
				Timestamp: now,
				Line:      fmt.Sprintf("log line %d from %s", i, app),
			})
			now = now.Add(time.Second)
		}

		pr.Streams = append(pr.Streams, push.Stream{
			Labels:  fmt.Sprintf(`{app="%s",content_type="protobuf"}`, app),
			Entries: entries,
		})
	}

	buf, err := pr.Marshal()
	if err != nil {
		return err
	}

	encoded := snappy.Encode(nil, buf)

	req, err := http.NewRequest(http.MethodPost, lokiAPIURL, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}
