package lokisourcegcplog

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const (
	testName        = "loki-source-gcplog"
	entriesPerShape = 10
	severity        = "INFO"
)

var logName = fmt.Sprintf("projects/%s/logs/integration-test", deps.GCPPubSubProject)

// gcpLogEntry is the subset of a Cloud Logging LogEntry the gcplog parser reads.
type gcpLogEntry struct {
	LogName  string `json:"logName"`
	Resource struct {
		Type   string            `json:"type"`
		Labels map[string]string `json:"labels"`
	} `json:"resource"`
	Timestamp   string `json:"timestamp"`
	Severity    string `json:"severity"`
	TextPayload string `json:"textPayload"`
}

type resourceShape struct {
	resourceType string
	labels       map[string]string
	// expected are the stream labels the shape produces, besides alloy_test_name.
	expected map[string]string
}

var shapes = []resourceShape{
	{
		resourceType: "k8s_cluster",
		labels:       map[string]string{"cluster_name": "dev-us-central-42", "location": "us-central1"},
		expected:     map[string]string{"service_name": "k8s_cluster", "cluster": "dev-us-central-42"},
	},
	{
		resourceType: "gcs_bucket",
		labels:       map[string]string{"bucket_name": "loki-bucket"},
		expected:     map[string]string{"service_name": "gcs_bucket", "bucket": "loki-bucket"},
	},
	{
		resourceType: "cloud_function",
		labels:       map[string]string{"function_name": "loki-fn", "region": "us-central1"},
		expected:     map[string]string{"service_name": "cloud_function"},
	},
}

// TestLokiSourceGcplog publishes Cloud Logging entries to a GCP Pub/Sub emulator
// topic. loki.source.gcplog must pull them from the subscription and apply its
// relabel rules.
func TestLokiSourceGcplog(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-loki-source-gcplog",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	gcpPubSub := deps.NewGCPPubSub(deps.GCPPubSubOptions{Namespace: ns.Name()})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-loki-source-gcplog",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, gcpPubSub, loki, alloy},
	})

	now := time.Now().UTC()
	var messages [][]byte
	expected := make([]deps.ExpectedLogResult, 0, len(shapes)+1)
	for _, s := range shapes {
		for i := range entriesPerShape {
			var e gcpLogEntry
			e.LogName = logName
			e.Resource.Type = s.resourceType
			e.Resource.Labels = s.labels
			e.Timestamp = now.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
			e.Severity = severity
			e.TextPayload = fmt.Sprintf("%s message %d", s.resourceType, i)

			b, err := json.Marshal(e)
			require.NoError(t, err)
			messages = append(messages, b)
		}
		expected = append(expected, deps.ExpectedLogResult{
			EntryCount:         entriesPerShape,
			Labels:             s.expected,
			StructuredMetadata: map[string]string{"severity": severity, "log_name": logName},
		})
	}
	// Nothing outside the published messages reaches Loki.
	expected = append(expected, deps.ExpectedLogResult{EntryCount: entriesPerShape * len(shapes)})

	gcpPubSub.Publish(t, messages...)

	loki.QueryLogs(t, testName, expected...)
	loki.QueryLabelsNotIndexed(t, testName, "severity", "log_name")
}
