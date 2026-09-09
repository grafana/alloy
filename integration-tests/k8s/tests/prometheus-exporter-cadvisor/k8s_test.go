package prometheusexportercadvisor

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestPrometheusExporterCadvisor(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-prometheus-exporter-cadvisor",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-prometheus-exporter-cadvisor",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, alloy},
	})

	// Covers the cAdvisor collector families: version, cpu, memory, filesystem,
	// network, and blkio. cadvisor_build_info comes from the Alloy integration
	// wrapper, not from cAdvisor.
	//
	// Two families are left out on purpose:
	//   - container_pressure_* (PSI) needs kernel CONFIG_PSI. Not every host has it.
	//   - container_health_state needs a container with a Docker HEALTHCHECK. It
	//     depends on the sibling workloads, not the exporter.
	mimir.QueryMetrics(t, "cadvisor", []string{
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
	})
}
