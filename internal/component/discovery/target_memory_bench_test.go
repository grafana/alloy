package discovery

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	commonlabels "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

// Benchmark_Targets_ResidentMemory measures the memory a live set of targets
// occupies once service discovery has handed them over, which is what matters
// for a collector holding tens of thousands of targets between updates.
//
// Per-operation allocs/op cannot show this. The map-based layout allocated almost
// nothing when building a target because it stored references to the LabelSets
// the Prometheus service discovery library had already built, but it kept every
// one of those maps resident for the lifetime of the target and made the GC scan
// them. The packed layout copies the labels once, which lets the source maps be
// collected.
//
// The measurement therefore drops the service discovery cache before sampling the
// heap: whatever the targets still reference stays resident.
//
// Run with -benchtime 1x, since the reported metrics are absolute rather than
// per-iteration.
func Benchmark_Targets_ResidentMemory(b *testing.B) {
	const (
		targetsCount = 100_000
		groupsCount  = 20 // as if 20 Kubernetes services were discovered
	)

	buildCache := func() map[string]*targetgroup.Group {
		cache := make(map[string]*targetgroup.Group, groupsCount)
		for g := 0; g < groupsCount; g++ {
			// Shared meta labels, as Kubernetes SD produces per target group.
			shared := commonlabels.LabelSet{}
			for i := 0; i < 20; i++ {
				shared[commonlabels.LabelName(fmt.Sprintf("__meta_kubernetes_service_label_%d", i))] =
					commonlabels.LabelValue(fmt.Sprintf("shared_value_%d_%d", g, i))
			}
			shared["job"] = commonlabels.LabelValue(fmt.Sprintf("kubernetes-pods-%d", g))

			targets := make([]commonlabels.LabelSet, 0, targetsCount/groupsCount)
			for i := 0; i < targetsCount/groupsCount; i++ {
				targets = append(targets, commonlabels.LabelSet{
					"__address__":                     commonlabels.LabelValue(fmt.Sprintf("10.132.%d.%d:8080", g, i%256)),
					"__meta_kubernetes_pod_name":      commonlabels.LabelValue(fmt.Sprintf("pod-%d-%d", g, i)),
					"__meta_kubernetes_pod_ip":        commonlabels.LabelValue(fmt.Sprintf("10.132.%d.%d", g, i%256)),
					"__meta_kubernetes_pod_node_name": commonlabels.LabelValue(fmt.Sprintf("node-%d", i%64)),
					"__meta_kubernetes_pod_uid":       commonlabels.LabelValue(fmt.Sprintf("4c586419-7f6c-448d-aeec-%012d", i)),
				})
			}

			source := fmt.Sprintf("group_%d", g)
			cache[source] = &targetgroup.Group{Source: source, Labels: shared, Targets: targets}
		}
		return cache
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	// Build the targets through the real service discovery path, then drop the
	// cache. Anything the targets still reference cannot be collected.
	cache := buildCache()
	targets := toAlloyTargets(cache)
	cache = nil
	_ = cache

	runtime.GC()
	runtime.GC() // second cycle so that anything freed by finalisation is accounted for
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if len(targets) != targetsCount {
		b.Fatalf("expected %d targets, got %d", targetsCount, len(targets))
	}

	residentBytes := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	residentObjects := int64(after.HeapObjects) - int64(before.HeapObjects)
	b.ReportMetric(float64(residentBytes)/targetsCount, "resident-B/target")
	b.ReportMetric(float64(residentObjects)/targetsCount, "resident-obj/target")

	// Measure how long a GC cycle takes with this target set live, which is driven
	// by the number of pointers the GC has to chase per target.
	const gcRuns = 10
	start := time.Now()
	for i := 0; i < gcRuns; i++ {
		runtime.GC()
	}
	b.ReportMetric(float64(time.Since(start).Nanoseconds())/gcRuns, "gc-ns/cycle")

	// Touch every target so none of the above can be optimised away, and so the
	// set is still live for the GC measurements.
	total := 0
	for i := range targets {
		total += targets[i].Len()
	}
	if want := targetsCount * 26; total != want {
		b.Fatalf("unexpected total label count: got %d, want %d", total, want)
	}
	runtime.KeepAlive(targets)
}
