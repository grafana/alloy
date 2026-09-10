//go:build alloyintegrationtests

package main

import (
	"runtime"
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestCadvisorHealthState(t *testing.T) {
	// cAdvisor only runs on Linux. It needs a real Linux cgroup tree.
	if runtime.GOOS != "linux" {
		t.Skip("Skipping cAdvisor health state test on non-Linux platform")
	}

	// container_health_state comes from the Docker API. cAdvisor v0.60.5 pulled in
	// google/cadvisor#3760, one change that fixed two bugs:
	//   - A container with no HEALTHCHECK reports -1, not 0 (issue #6926).
	//   - The value refreshes on every scrape, so it no longer stays frozen at its
	//     start-time snapshot (issue #6928).
	testName := "cadvisor_health"

	// The -1 result proves the fix landed. Before #3760 a container with no
	// HEALTHCHECK reported 0, indistinguishable from an unhealthy container. Because
	// both bugs were the same upstream change, this also confirms the refresh code.
	common.AssertMetricValue(t,
		common.ContainerMetricQuery("container_health_state", testName, "cadvisor-health-nohealthcheck"),
		"container_health_state", "-1")

	// The healthy sibling reports 1. The harness starts it (and waits for healthy)
	// before Alloy, so this guards the healthy path but does not by itself prove the
	// refresh fix: the pre-fix snapshot would also catch an already-healthy container.
	common.AssertMetricValue(t,
		common.ContainerMetricQuery("container_health_state", testName, "cadvisor-health-healthy"),
		"container_health_state", "1")
}

func TestCadvisorFilesystemMetrics(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping cAdvisor filesystem metrics test on non-Linux platform")
	}

	// cAdvisor v0.60 moved filesystem-plugin selection out of a process-global
	// registry. Alloy now passes the plugin set to the manager explicitly, so the
	// container_fs_* metrics are populated again. In a Docker environment cAdvisor
	// resolves each container's filesystem, so usage and limit report per container.
	testName := "cadvisor_health"
	for _, metric := range []string{"container_fs_usage_bytes", "container_fs_limit_bytes"} {
		common.AssertMetricData(t,
			common.ContainerMetricQuery(metric, testName, "cadvisor-health-healthy"),
			metric, testName)
	}
}

func TestCadvisorApplicationMetrics(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping cAdvisor application metrics test on non-Linux platform")
	}

	// cAdvisor collects application metrics from a container that declares a
	// collector through io.cadvisor.metric.* labels. v0.60 makes the collector
	// manager an injection seam that the lean library leaves nil; Alloy now wires
	// CollectorManagerFactory. The cadvisor-appmetrics fixture serves a Prometheus
	// endpoint and declares it via an image label, so cAdvisor scrapes it and the
	// value surfaces as a custom metric. This exercises that wiring end to end.
	common.AssertMetricData(t,
		common.MetricQuery("test_app_metric", "cadvisor_health"),
		"test_app_metric", "cadvisor_health")
}
