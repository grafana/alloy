package prometheusexportercadvisor

import (
	"os"
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
	// network, and blkio, plus container_start_time_seconds added in v0.60.
	//
	// container_health_state is left out on purpose: it needs a container with a
	// Docker HEALTHCHECK, which kind's containerd runtime does not provide.
	expected := []string{
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
		// container_start_time_seconds was added in cAdvisor v0.60 and does not
		// depend on the cgroup version.
		"container_start_time_seconds",
	}

	// PSI and cgroup v2 memory metrics are only produced on capable kernels. kind
	// shares the host kernel, so gate on the host's capabilities and assert these
	// only where cAdvisor actually emits them, keeping the test portable to hosts
	// without CONFIG_PSI or cgroup v2 (e.g. local dev).
	if psiAvailable() {
		expected = append(expected,
			"container_pressure_cpu_stalled_seconds_total",
			"container_pressure_cpu_waiting_seconds_total",
			"container_pressure_io_stalled_seconds_total",
			"container_pressure_io_waiting_seconds_total",
			"container_pressure_memory_stalled_seconds_total",
			"container_pressure_memory_waiting_seconds_total",
		)
	} else {
		t.Log("skipping PSI metric assertions: /proc/pressure/cpu is not present")
	}
	if cgroupV2() {
		expected = append(expected,
			"container_memory_events_high_total",
			"container_memory_events_max_total",
			"container_memory_pgscan_total",
			"container_memory_pgsteal_total",
			"container_memory_workingset_refault_anon_total",
			"container_memory_workingset_refault_file_total",
		)
	} else {
		t.Log("skipping cgroup v2 memory metric assertions: cgroup v2 was not detected")
	}

	mimir.QueryMetrics(t, "cadvisor", expected)

	// The filesystem metrics must report real data, not just be present. These
	// come from the root cgroup's machine filesystem, so they are always > 0 and
	// prove the explicit filesystem-plugin wiring produces values.
	mimir.QueryPositive(t, "cadvisor", []string{
		"container_fs_usage_bytes",
		"container_fs_limit_bytes",
	})

	// The containerd socket lets cAdvisor resolve pod metadata, so container
	// labels are attached. Kubernetes sets these io.kubernetes.* labels on every
	// container, so container_last_seen carries all three when the plugin works.
	mimir.QueryMetricWithLabelsPresent(t, "cadvisor", "container_last_seen",
		"container_label_io_kubernetes_pod_name",
		"container_label_io_kubernetes_pod_namespace",
		"container_label_io_kubernetes_container_name",
	)
}

// psiAvailable reports whether the kernel exposes pressure stall information.
// The kind node shares the host kernel, so the host's /proc reflects the node.
func psiAvailable() bool {
	_, err := os.Stat("/proc/pressure/cpu")
	return err == nil
}

// cgroupV2 reports whether the host uses the cgroup v2 unified hierarchy.
func cgroupV2() bool {
	_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	return err == nil
}
