package clustering

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/stretchr/testify/require"
)

const (
	clusteringRelease = "alloy-test-clustering"
	// Number of prom-gen pods (the fixed workload). Independent of how many Alloy
	// instances share the scraping.
	promGenReplicas = 12
)

// TestClusteringActiveSeriesInvariant checks the core promise of clustering: the
// total active series across the cluster equals the true number of workload
// series, no matter how many Alloy instances share the work. It scales the
// cluster through several sizes and, once each change settles, asserts:
//
//	sum(prometheus_remote_write_wal_storage_active_series{workload}) == count(real workload series)
//
// The right-hand side is Mimir's deduped series count — the real number we know
// exists. If any target is scraped by more than one pod, the Alloy-side sum
// exceeds it and the test fails; if targets are dropped, coverage fails. The WAL
// truncation is set to 1m (in config.alloy) so active-series settles within a
// minute or two of each scale change instead of the 2h default.
func TestClusteringActiveSeriesInvariant(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-clustering",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	promGen := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Namespace: ns.Name(),
		Path:      "./config/prom-gen-cluster.yaml",
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    clusteringRelease,
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, promGen, alloy},
	})

	const (
		// Total workload active series across the cluster, as Alloy reports it.
		activeSeries = `sum(prometheus_remote_write_wal_storage_active_series{component_id="prometheus.remote_write.workload"})`
		// The real number of workload series, deduped by Mimir = ground truth.
		realSeries = `count({alloy_test_name="workload"})`
		// Every workload target is scraped (coverage / no gaps).
		coverage = `count(up{alloy_test_name="workload"})`
	)

	for _, replicas := range []int{3, 5, 10, 3} {
		t.Logf("scaling Alloy cluster to %d instances", replicas)
		scaleAlloy(t, ns.Name(), clusteringRelease, replicas)

		// Coverage: every workload target is still scraped.
		mimir.QueryEquals(t, coverage, promGenReplicas)

		// Invariant: total active series equals the real (deduped) series count,
		// regardless of instance count. Generous timeout for WAL settling.
		mimir.QueryEqualsQuery(t, activeSeries, realSeries, 6*time.Minute,
			fmt.Sprintf("active series must equal the real series count at %d instances", replicas))
	}
}

// scaleAlloy scales the Alloy StatefulSet and waits for the rollout to complete.
func scaleAlloy(t *testing.T, namespace, release string, replicas int) {
	t.Helper()
	require.NoError(t, harness.RunCommand("kubectl", "-n", namespace, "scale",
		"statefulset", release, fmt.Sprintf("--replicas=%d", replicas)))
	require.NoError(t, harness.RunCommand("kubectl", "-n", namespace, "rollout", "status",
		"statefulset", release, "--timeout=300s"))
}
