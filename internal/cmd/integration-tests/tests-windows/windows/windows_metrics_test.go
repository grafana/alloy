//go:build windows

package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestWindowsMetrics(t *testing.T) {
	var winMetrics = []string{
		"windows_cpu_logical_processor",    // cpu
		"windows_cs_logical_processors",    // cs
		"windows_logical_disk_info",        // logical_disk
		"windows_net_bytes_received_total", // net
		"windows_os_info",                  // os
		"windows_service_info",             // service
		"windows_system_processes",         // system
	}
	common.MimirMetricsTest(t, winMetrics, []string{}, "win_metrics")
}
