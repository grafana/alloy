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
