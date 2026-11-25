package prometheusexporterkafka

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const testName = "prometheus-exporter-kafka"

var kafkaMetrics = []string{
	"kafka_broker_info",
	"kafka_brokers",
	// kafka_consumer_lag_extrapolation is not reported, because the exporter
	// uses interpolation here.
	"kafka_consumer_lag_interpolation",
	"kafka_consumer_lag_millis",
	"kafka_consumergroup_current_offset",
	"kafka_consumergroup_current_offset_sum",
	"kafka_consumergroup_members",
	"kafka_consumergroup_uncommitted_offsets",
	"kafka_consumergroup_uncommitted_offsets_sum",
	"kafka_exporter_build_info",
	"kafka_topic_partition_current_offset",
	"kafka_topic_partition_in_sync_replica",
	"kafka_topic_partition_leader",
	"kafka_topic_partition_leader_is_preferred",
	"kafka_topic_partition_oldest_offset",
	"kafka_topic_partition_replicas",
	"kafka_topic_partition_under_replicated_partition",
	"kafka_topic_partitions",
}

// TestPrometheusExporterKafka keeps a consumer group busy on a real broker.
// prometheus.exporter.kafka must report broker, topic and consumer group
// metrics.
func TestPrometheusExporterKafka(t *testing.T) {
	const topic = "test_topic"

	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-prometheus-exporter-kafka",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	kafka := deps.NewKafka(deps.KafkaOptions{
		Namespace: ns.Name(),
		Topics:    []string{topic},
	})
	kafkaGen := deps.NewKafkaGen(deps.KafkaGenOptions{
		Kafka: kafka,
		Topic: topic,
		Group: "test_consumer_group",
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-prometheus-exporter-kafka",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, kafka, kafkaGen, mimir, alloy},
	})

	mimir.QueryMetrics(t, testName, kafkaMetrics)
}
