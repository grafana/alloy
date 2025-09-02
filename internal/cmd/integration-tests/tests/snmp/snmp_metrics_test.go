package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestSNMPMetrics(t *testing.T) {
	var SNMPMetrics = []string{
		"scrape_duration_seconds",
		"scrape_samples_post_metric_relabeling",
		"scrape_samples_scraped",
		"scrape_series_added",
		"snmp_packet_duration_seconds_bucket",
		"snmp_packet_duration_seconds_count",
		"snmp_packet_duration_seconds_sum",
		"snmp_packet_retries_total",
		"snmp_packets_total",
		"snmp_request_in_flight",
		"snmp_scrape_duration_seconds",
		"snmp_scrape_packets_retried",
		"snmp_scrape_packets_sent",
		"snmp_scrape_pdus_returned",
		"snmp_scrape_walk_duration_seconds",
		"snmp_unexpected_pdu_type_total",
		"sysDescr",
		"up",
	}
	var SNMPMetrics3 = []string{
		"scrape_duration_seconds",
		"scrape_samples_post_metric_relabeling",
		"scrape_samples_scraped",
		"scrape_series_added",
		"snmp_packet_duration_seconds_bucket",
		"snmp_packet_duration_seconds_count",
		"snmp_packet_duration_seconds_sum",
		"snmp_packet_retries_total",
		"snmp_packets_total",
		"snmp_request_in_flight",
		"snmp_scrape_duration_seconds",
		"snmp_scrape_packets_retried",
		"snmp_scrape_packets_sent",
		"snmp_scrape_pdus_returned",
		"snmp_scrape_walk_duration_seconds",
		"snmp_unexpected_pdu_type_total",
		"sysDescr",
		"hrDeviceIndex",
		"up",
	}
	var SNMPMetrics4 = []string{
		"scrape_duration_seconds",
		"scrape_samples_post_metric_relabeling",
		"scrape_samples_scraped",
		"scrape_series_added",
		"snmp_packet_duration_seconds_bucket",
		"snmp_packet_duration_seconds_count",
		"snmp_packet_duration_seconds_sum",
		"snmp_packet_retries_total",
		"snmp_packets_total",
		"snmp_request_in_flight",
		"snmp_scrape_duration_seconds",
		"snmp_scrape_packets_retried",
		"snmp_scrape_packets_sent",
		"snmp_scrape_pdus_returned",
		"snmp_scrape_walk_duration_seconds",
		"snmp_unexpected_pdu_type_total",
		"sysDescr",
		"hrDeviceID",
		"up",
	}
	common.MimirMetricsTest(common.Config{
		T:        t,
		TestName: "snmp_metrics",
		Metrics:  SNMPMetrics,
	})
	common.MimirMetricsTest(common.Config{
		T:        t,
		TestName: "snmp_metrics2",
		Metrics:  SNMPMetrics,
	})
	common.MimirMetricsTest(common.Config{
		T:        t,
		TestName: "snmp_metrics3",
		Metrics:  SNMPMetrics3,
	})
	common.MimirMetricsTest(common.Config{
		T:        t,
		TestName: "snmp_metrics4",
		Metrics:  SNMPMetrics4,
	})
}
