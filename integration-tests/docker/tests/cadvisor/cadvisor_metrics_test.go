//go:build alloyintegrationtests

package main

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cadvisorPrefixes are the metric name prefixes cAdvisor's collectors emit.
var cadvisorPrefixes = []string{"machine_", "container_", "cadvisor_"}

// TestCadvisorMetrics asserts a stable core of cAdvisor metrics reach Mimir.
//
// The assertion list is not pinned yet. It is skipped until a real run on Linux
// tells us which metrics land reliably. Use TestCadvisorDiscoverMetrics to get
// that list, then fill it in here and remove the skip and the discovery test.
func TestCadvisorMetrics(t *testing.T) {
	t.Skip("TEMPORARY: pin expectedMetrics from TestCadvisorDiscoverMetrics output, then remove this skip")

	if runtime.GOOS != "linux" {
		t.Skip("Skipping cAdvisor metrics test on non-Linux platform")
	}

	expectedMetrics := []string{
		// Filled in from a real run.
	}

	common.MimirMetricsTest(t, expectedMetrics, []string{}, "cadvisor_metrics")
}

// TestCadvisorDiscoverMetrics is a TEMPORARY POC helper. It lists every
// cAdvisor-prefixed metric that reached Mimir, sorted and formatted as a
// paste-ready Go slice. It fails on purpose. The harness prints test output
// only for failing tests, so failing is how we surface the list.
//
// Remove this test once TestCadvisorMetrics has a real assertion list.
func TestCadvisorDiscoverMetrics(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping cAdvisor metrics discovery on non-Linux platform")
	}
	common.AssertStatefulTestEnv(t)

	var found []string
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var resp common.MetricsResponse
		_, err := common.FetchDataFromURL(common.MetricsQuery("cadvisor_metrics"), &resp)
		assert.NoError(c, err)

		names := map[string]struct{}{}
		for _, m := range resp.Data {
			for _, p := range cadvisorPrefixes {
				if strings.HasPrefix(m.Name, p) {
					names[m.Name] = struct{}{}
					break
				}
			}
		}

		found = found[:0]
		for name := range names {
			found = append(found, name)
		}
		// Wait for a reasonable set before reporting, so the list is not cut short.
		assert.GreaterOrEqual(c, len(found), 5, "waiting for cAdvisor metrics to appear")
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)

	sort.Strings(found)

	var b strings.Builder
	for _, name := range found {
		fmt.Fprintf(&b, "\t\t%q,\n", name)
	}
	t.Errorf("TEMPORARY cAdvisor metric discovery — %d metrics found. Copy into TestCadvisorMetrics:\n%s", len(found), b.String())
}
