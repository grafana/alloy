//go:build alloyintegrationtests && !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

// PRWv2 = prometheus remote write v2 format https://prometheus.io/docs/specs/prw/remote_write_spec_2_0/
func TestScrapePromMetricsPRWv2(t *testing.T) {
	common.MimirMetricsTest(t, common.PromDefaultMetrics, nil, "scrape_prom_metrics_prw_v2")
}
