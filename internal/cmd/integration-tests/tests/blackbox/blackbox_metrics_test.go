//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestBlackBoxMetrics(t *testing.T) {
	var blackboxMetrics = []string{
		"probe_dns_lookup_time_seconds",
		"probe_duration_seconds",
		"probe_failed_due_to_regex",
		"probe_http_content_length",
		"probe_http_duration_seconds",
		"probe_http_redirects",
		"probe_http_ssl",
		"probe_http_status_code",
		"probe_http_uncompressed_body_length",
		"probe_http_version",
		"probe_ip_addr_hash",
		"probe_ip_protocol",
		"probe_success",
		"scrape_duration_seconds",
		"scrape_samples_post_metric_relabeling",
		"scrape_samples_scraped",
		"scrape_series_added",
		"up",
	}
	common.MimirMetricsTest(t, blackboxMetrics, []string{}, "blackbox_metrics")
}
