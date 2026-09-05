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

// Benchmark_ToAlloyTargets_Memoised models what a long-running component
// actually does: many discovered sources, of which only one changes between
// sends. runDiscovery keeps the *targetgroup.Group for every unchanged source, so
// without memoisation each send re-sorts and re-encodes the labels of every
// target of every source.
//
// Compare against Benchmark_ToAlloyTargets, which measures the cold conversion.
//
// 50 sources x 400 targets = 20k targets, on linux/amd64 Ryzen AI MAX+ 395:
//
//	                                    time/op    B/op    allocs/op
//	map-based layout (before packing)    2.30ms    484kB        2
//	packed, no memoisation               4.28ms   4224kB    20159
//	packed, memoised, 1 source changes   0.39ms    900kB     3825
//	packed, memoised, nothing changes    0.045ms   648kB        2
//
// The memoised numbers are the ones that matter for a long-running collector.
// Note the worst case is unchanged by memoisation: a discoverer with a single
// source that changes on every cycle still pays the full conversion, which is
// what Benchmark_ToAlloyTargets measures.
func Benchmark_ToAlloyTargets_Memoised(b *testing.B) {
	const (
		sources          = 50
		targetsPerSource = 400 // 20k targets in total
	)

	mkGroup := func(s, gen int) *targetgroup.Group {
		shared := commonlabels.LabelSet{
			"job":                               commonlabels.LabelValue(fmt.Sprintf("job_%d", s)),
			"__meta_kubernetes_namespace":       "prod",
			"__meta_kubernetes_service_name":    commonlabels.LabelValue(fmt.Sprintf("svc_%d", s)),
			"__meta_kubernetes_service_label_a": "aaaaaaaaaa",
			"__meta_kubernetes_service_label_b": "bbbbbbbbbb",
		}
		targets := make([]commonlabels.LabelSet, 0, targetsPerSource)
		for i := 0; i < targetsPerSource; i++ {
			targets = append(targets, commonlabels.LabelSet{
				"__address__":                commonlabels.LabelValue(fmt.Sprintf("10.%d.%d.%d:8080", s%256, i%256, gen%256)),
				"__meta_kubernetes_pod_name": commonlabels.LabelValue(fmt.Sprintf("pod-%d-%d-%d", s, i, gen)),
				"__meta_kubernetes_pod_ip":   commonlabels.LabelValue(fmt.Sprintf("10.%d.%d.%d", s%256, i%256, gen%256)),
				"__meta_kubernetes_pod_uid":  commonlabels.LabelValue(fmt.Sprintf("uid-%d-%d-%d", s, i, gen)),
			})
		}
		return &targetgroup.Group{Source: fmt.Sprintf("source_%d", s), Labels: shared, Targets: targets}
	}

	cache := make(map[string]*targetgroup.Group, sources)
	for s := 0; s < sources; s++ {
		cache[fmt.Sprintf("source_%d", s)] = mkGroup(s, 0)
	}

	b.Run("one source changes per send", func(b *testing.B) {
		packer := newGroupPacker()
		packer.toAlloyTargets(cache) // warm
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s := i % sources
			cache[fmt.Sprintf("source_%d", s)] = mkGroup(s, i+1)
			_ = packer.toAlloyTargets(cache)
		}
	})

	b.Run("nothing changes", func(b *testing.B) {
		packer := newGroupPacker()
		packer.toAlloyTargets(cache) // warm
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = packer.toAlloyTargets(cache)
		}
	})

	b.Run("no memoisation", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = toAlloyTargets(cache)
		}
	})
}
