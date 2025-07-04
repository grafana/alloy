//go:build !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestScrapePromMetricsToOtlp(t *testing.T) {
	// TODO(thampiotr): Histograms are not working in `otelcol.receiver.prometheus` because metadata is not implemented.
	// Removing the check for common.PromDefaultHistogramMetric until it's fixed.
	// common.MimirMetricsTest(t, common.PromDefaultMetrics, common.PromDefaultHistogramMetric, "scrape_prom_metrics_to_otlp")
	common.MimirMetricsTest(t, common.PromDefaultMetrics, []string{}, "scrape_prom_metrics_to_otlp")
}
