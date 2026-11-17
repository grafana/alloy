package lokisourceapi

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

	"github.com/grafana/alloy/integration-tests/internal/lokihttp"
	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const (
	testName    = "loki-source-api"
	lokiAPIPort = 1515
)

func TestLokiSourceAPI(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-loki-source-api",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:    ns.Name(),
		Release:      "alloy-test-loki-source-api",
		ConfigPath:   "./config/config.alloy",
		ValuesPath:   "./config/alloy-values.yaml",
		ForwardPorts: []int{lokiAPIPort},
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, loki, alloy},
	})

	pushURL, err := alloy.Endpoint(lokiAPIPort, "/loki/api/v1/push")
	require.NoError(t, err)

	require.NoError(t, pushJSON(pushURL, "frontend", "backend"))
	require.NoError(t, pushProto(pushURL, "frontend-proto", "backend-proto"))

	loki.QueryLogs(t, testName,
		deps.ExpectedLogResult{
			EntryCount:         30,
			Labels:             map[string]string{"service_name": "frontend"},
			StructuredMetadata: map[string]string{"content_type": "json"},
		},
		deps.ExpectedLogResult{
			EntryCount:         30,
			Labels:             map[string]string{"service_name": "backend"},
			StructuredMetadata: map[string]string{"content_type": "json"},
		},
		deps.ExpectedLogResult{
			EntryCount:         30,
			Labels:             map[string]string{"service_name": "frontend-proto"},
			StructuredMetadata: map[string]string{"content_type": "protobuf"},
		},
		deps.ExpectedLogResult{
			EntryCount:         30,
			Labels:             map[string]string{"service_name": "backend-proto"},
			StructuredMetadata: map[string]string{"content_type": "protobuf"},
		},
	)

	loki.QueryLabelsNotIndexed(t, testName, "app")
}

func pushJSON(pushURL string, apps ...string) error {
	var (
		now     = time.Now()
		streams = make([]lokihttp.LogData, 0, len(apps))
	)

	for _, app := range apps {
		values := make([]lokihttp.LogEntry, 0, 30)
		for i := range 30 {
			values = append(values, lokihttp.LogEntry{
				Timestamp: fmt.Sprintf("%d", now.UnixNano()),
				Line:      fmt.Sprintf("log line %d from %s", i, app),
			})
			now = now.Add(time.Second)
		}

		streams = append(streams, lokihttp.LogData{
			Stream: map[string]string{
				"app":          app,
				"content_type": "json",
			},
			Values: values,
		})
	}

	body, err := json.Marshal(lokihttp.PushRequest{Streams: streams})
	if err != nil {
		return err
	}

	resp, err := http.Post(pushURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func pushProto(pushURL string, apps ...string) error {
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

	req, err := http.NewRequest(http.MethodPost, pushURL, bytes.NewReader(encoded))
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
