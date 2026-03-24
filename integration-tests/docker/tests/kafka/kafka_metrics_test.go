//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestKafkaMetrics(t *testing.T) {
	var kafkaMetrics = []string{
		"kafka_topic_partition_replicas",
		"kafka_topic_partition_leader",
		"kafka_exporter_build_info",
		"kafka_consumergroup_current_offset",
		"kafka_consumergroup_current_offset_sum",
		// "kafka_consumer_lag_extrapolation", this one is not generated because it uses "kafka_consumer_lag_interpolation" instead
		"kafka_consumer_lag_interpolation",
		"kafka_broker_info",
		"kafka_brokers",
		"kafka_consumergroup_uncommitted_offsets_sum",
		"kafka_topic_partition_current_offset",
		"kafka_consumergroup_uncommitted_offsets",
		"kafka_topic_partition_in_sync_replica",
		"kafka_topic_partition_under_replicated_partition",
		"kafka_topic_partition_leader_is_preferred",
		"kafka_consumergroup_members",
		"kafka_consumer_lag_millis",
		"kafka_topic_partitions",
		"kafka_topic_partition_oldest_offset",
	}
	common.MimirMetricsTest(t, kafkaMetrics, []string{}, "kafka_metrics")
}
