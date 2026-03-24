//go:build alloyintegrationtests && !windows

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestScrapePromMetricsRemoteWrite(t *testing.T) {
	common.MimirMetricsTest(t, common.PromDefaultMetrics, nil, "scrape_prom_metrics_remote_write")
}
