package lokisourcekafka

import (
	"strconv"
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const (
	testName         = "loki-source-kafka"
	messagesPerTopic = 50
	partitions       = 3
)

var topics = []string{"test1", "test2", "test3"}

// TestLokiSourceKafka produces keyed records to several multi-partition
// topics. loki.source.kafka must consume every record and label it with its
// topic.
func TestLokiSourceKafka(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-loki-source-kafka",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	kafka := deps.NewKafka(deps.KafkaOptions{
		Namespace:  ns.Name(),
		Topics:     topics,
		Partitions: partitions,
	})
	loki := deps.NewLoki(deps.LokiOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-loki-source-kafka",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, kafka, loki, alloy},
	})

	// Distinct keys spread the records across every partition.
	records := make([]deps.KafkaRecord, 0, messagesPerTopic)
	for i := range messagesPerTopic {
		records = append(records, deps.KafkaRecord{
			Key:   strconv.Itoa(i),
			Value: "log line " + strconv.Itoa(i),
		})
	}

	expected := make([]deps.ExpectedLogResult, 0, len(topics)+1)
	for _, topic := range topics {
		kafka.ProduceKeyed(t, topic, records...)
		expected = append(expected, deps.ExpectedLogResult{
			EntryCount: messagesPerTopic,
			Labels:     map[string]string{"service_name": topic},
		})
	}
	// Nothing outside the produced records reaches Loki.
	expected = append(expected, deps.ExpectedLogResult{EntryCount: messagesPerTopic * len(topics)})

	loki.QueryLogs(t, testName, expected...)
}
