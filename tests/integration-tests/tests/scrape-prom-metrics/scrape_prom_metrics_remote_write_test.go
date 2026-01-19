//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/tests/integration-tests/common"
)

func TestScrapePromMetricsRemoteWrite(t *testing.T) {
	common.MimirMetricsTest(t, common.PromDefaultMetrics, nil, "scrape_prom_metrics_remote_write")
}
