//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestScrapePromMetricsToOtlp(t *testing.T) {
	config := common.Config{
		T:                t,
		TestName:         "scrape_prom_metrics_to_otlp",
		Metrics:          common.PromDefaultMetrics,
		HistogramMetrics: common.PromDefaultNativeHistogramMetrics,
		ExpectedMetadata: common.ExpectedMetadata,
	}
	common.MimirMetricsTest(config)
}
