//go:build alloyintegrationtests && !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestScrapePromMetricsWriteQueue(t *testing.T) {
	common.MimirMetricsTest(t, common.PromDefaultMetrics, common.PromDefaultNativeHistogramMetrics, "scrape_prom_metrics_writequeue")
}
