//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestScrapePromMetricsWriteQueue(t *testing.T) {
	common.MimirMetricsTest(t, common.PromDefaultMetrics, common.PromDefaultNativeHistogramMetrics, "scrape_prom_metrics_writequeue")
}
