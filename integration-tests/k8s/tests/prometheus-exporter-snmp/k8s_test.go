package prometheusexportersnmp

import (
	"slices"
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// Metrics every exporter in the config reports, whatever its modules.
var commonMetrics = []string{
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

func TestPrometheusExporterSNMP(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-prometheus-exporter-snmp",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	snmpSim := deps.NewSNMPSim(deps.SNMPSimOptions{Namespace: ns.Name()})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-prometheus-exporter-snmp",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, snmpSim, mimir, alloy},
	})

	mimir.QueryMetrics(t, "snmp-targets", commonMetrics)
	mimir.QueryMetrics(t, "snmp-target-block", commonMetrics)
	mimir.QueryMetrics(t, "snmp-multi-module", slices.Concat(commonMetrics, []string{"hrDeviceIndex"}))
	mimir.QueryMetrics(t, "snmp-merge", slices.Concat(commonMetrics, []string{"hrDeviceID"}))
}
