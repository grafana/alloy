//go:build alloyintegrationtests

package main

import (
	"runtime"
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestCadvisorMetrics(t *testing.T) {
	// cAdvisor only runs on Linux. The test exercises the Grafana cAdvisor fork's
	// collectors, so it must run against a real Linux cgroup tree.
	if runtime.GOOS != "linux" {
		t.Skip("Skipping cAdvisor metrics test on non-Linux platform")
	}

	// Pinned from a real CI run. This covers every cAdvisor collector family:
	// build/version, cpu, memory, filesystem, network, and blkio.
	//
	// Two families are left out on purpose:
	//   - container_pressure_* (PSI) needs kernel CONFIG_PSI and is not present
	//     on every host.
	//   - container_health_state is emitted only for containers with a Docker
	//     HEALTHCHECK, so it depends on the sibling images, not the exporter.
	expectedMetrics := []string{
		"cadvisor_build_info",
		"cadvisor_version_info",
		"container_blkio_device_usage_total",
		"container_cpu_load_average_10s",
		"container_cpu_load_d_average_10s",
		"container_cpu_system_seconds_total",
		"container_cpu_usage_seconds_total",
		"container_cpu_user_seconds_total",
		"container_fs_inodes_free",
		"container_fs_inodes_total",
		"container_fs_io_current",
		"container_fs_io_time_seconds_total",
		"container_fs_io_time_weighted_seconds_total",
		"container_fs_limit_bytes",
		"container_fs_read_seconds_total",
		"container_fs_reads_bytes_total",
		"container_fs_reads_merged_total",
		"container_fs_reads_total",
		"container_fs_sector_reads_total",
		"container_fs_sector_writes_total",
		"container_fs_usage_bytes",
		"container_fs_write_seconds_total",
		"container_fs_writes_bytes_total",
		"container_fs_writes_merged_total",
		"container_fs_writes_total",
		"container_last_seen",
		"container_memory_cache",
		"container_memory_failcnt",
		"container_memory_failures_total",
		"container_memory_kernel_usage",
		"container_memory_mapped_file",
		"container_memory_max_usage_bytes",
		"container_memory_rss",
		"container_memory_swap",
		"container_memory_total_active_file_bytes",
		"container_memory_total_inactive_file_bytes",
		"container_memory_usage_bytes",
		"container_memory_working_set_bytes",
		"container_network_receive_bytes_total",
		"container_network_receive_errors_total",
		"container_network_receive_packets_dropped_total",
		"container_network_receive_packets_total",
		"container_network_transmit_bytes_total",
		"container_network_transmit_errors_total",
		"container_network_transmit_packets_dropped_total",
		"container_network_transmit_packets_total",
		"container_oom_events_total",
	}

	common.MimirMetricsTest(t, expectedMetrics, []string{}, "cadvisor_metrics")
}
