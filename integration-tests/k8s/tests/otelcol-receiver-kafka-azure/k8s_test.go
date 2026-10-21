package otelcolreceiverkafkaazure

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const (
	testName = "otelcol-receiver-kafka-azure"
	topic    = "azure-resource-logs"

	// Each message is an Event Hubs batch with one record per resource.
	messageCount = 10

	region       = "westeurope"
	keyVaultID   = "/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.KEYVAULT/VAULTS/KV"
	storageID    = "/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.STORAGE/STORAGEACCOUNTS/SA"
	auditEvent   = "AuditEvent"
	secretGetOp  = "SecretGet"
	blobWriteOp  = "PutBlob"
	storageWrite = "StorageWrite"
)

// TestOtelcolReceiverKafkaAzure produces Azure resource logs, in the format
// Diagnostic Settings stream to Event Hubs, into Kafka. otelcol.receiver.kafka
// must decode them with the otelcol.encoding.azure extension.
func TestOtelcolReceiverKafkaAzure(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-otelcol-receiver-kafka-azure",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	kafka := deps.NewKafka(deps.KafkaOptions{
		Namespace: ns.Name(),
		Topics:    []string{topic},
	})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-otelcol-receiver-kafka-azure",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, kafka, loki, alloy},
	})

	messages := make([]string, 0, messageCount)
	now := time.Now().UTC()
	for i := range messageCount {
		ts := now.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
		messages = append(messages, eventHubsBatch(t,
			resourceLog{
				Time:          ts,
				ResourceID:    keyVaultID,
				Category:      auditEvent,
				OperationName: secretGetOp,
				Location:      region,
			},
			resourceLog{
				Time:          ts,
				ResourceID:    storageID,
				Category:      storageWrite,
				OperationName: blobWriteOp,
				Location:      region,
			},
		))
	}
	kafka.Produce(t, topic, messages...)

	loki.QueryLogs(t, testName,
		deps.ExpectedLogResult{
			EntryCount: messageCount,
			Labels: map[string]string{
				"cloud_provider":       "azure",
				"cloud_region":         region,
				"cloud_resource_id":    keyVaultID,
				"azure_category":       auditEvent,
				"azure_operation_name": secretGetOp,
			},
		},
		deps.ExpectedLogResult{
			EntryCount: messageCount,
			Labels: map[string]string{
				"cloud_provider":       "azure",
				"cloud_region":         region,
				"cloud_resource_id":    storageID,
				"azure_category":       storageWrite,
				"azure_operation_name": blobWriteOp,
			},
		},
	)
}

// resourceLog is the subset of the Azure resource log common schema the test
// relies on.
type resourceLog struct {
	Time          string `json:"time"`
	ResourceID    string `json:"resourceId"`
	Category      string `json:"category"`
	OperationName string `json:"operationName"`
	Location      string `json:"location"`
}

// eventHubsBatch wraps records the way Diagnostic Settings send them to
// Event Hubs: a single JSON object with a "records" array.
func eventHubsBatch(t *testing.T, records ...resourceLog) string {
	t.Helper()

	b, err := json.Marshal(struct {
		Records []resourceLog `json:"records"`
	}{Records: records})
	require.NoError(t, err)
	return string(b)
}
