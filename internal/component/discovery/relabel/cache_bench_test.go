package relabel

import (
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	commonlabels "github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
)

func benchRegexp(tb testing.TB, s string) alloy_relabel.Regexp {
	tb.Helper()
	var re alloy_relabel.Regexp
	require.NoError(tb, re.UnmarshalText([]byte(s)))
	return re
}

func benchRule(c alloy_relabel.Config) *alloy_relabel.Config {
	out := alloy_relabel.DefaultRelabelConfig
	if len(c.SourceLabels) > 0 {
		out.SourceLabels = c.SourceLabels
	}
	if c.Separator != "" {
		out.Separator = c.Separator
	}
	if c.Regex.Regexp != nil {
		out.Regex = c.Regex
	}
	if c.Modulus != 0 {
		out.Modulus = c.Modulus
	}
	if c.TargetLabel != "" {
		out.TargetLabel = c.TargetLabel
	}
	if c.Replacement != "" {
		out.Replacement = c.Replacement
	}
	if c.Action != "" {
		out.Action = c.Action
	}
	return &out
}

// benchRules is a realistic Kubernetes pod rule set: it inspects labels, maps
// some, computes a hashmod and drops a temporary label.
func benchRules(tb testing.TB) []*alloy_relabel.Config {
	tb.Helper()
	return []*alloy_relabel.Config{
		benchRule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_pod_phase"},
			Regex:        benchRegexp(tb, "Running"),
			Action:       alloy_relabel.Keep,
		}),
		benchRule(alloy_relabel.Config{
			SourceLabels: []string{"__address__", "__meta_kubernetes_pod_annotation_prometheus_io_port"},
			Regex:        benchRegexp(tb, `(.+?)(\:\d+)?;(\d+)`),
			TargetLabel:  "__address__",
			Replacement:  "$1:$3",
		}),
		benchRule(alloy_relabel.Config{
			Regex:       benchRegexp(tb, "__meta_kubernetes_pod_label_(.+)"),
			Action:      alloy_relabel.LabelMap,
			Replacement: "$1",
		}),
		benchRule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_namespace", "__meta_kubernetes_pod_name"},
			Separator:    "/",
			TargetLabel:  "instance",
		}),
		benchRule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_namespace"},
			TargetLabel:  "namespace",
		}),
		benchRule(alloy_relabel.Config{
			TargetLabel: "cluster",
			Replacement: "dev-us-central-0",
		}),
		benchRule(alloy_relabel.Config{
			SourceLabels: []string{"__address__"},
			TargetLabel:  "__tmp_hash",
			Modulus:      3,
			Action:       alloy_relabel.HashMod,
		}),
		benchRule(alloy_relabel.Config{
			Regex:  benchRegexp(tb, "__tmp_hash"),
			Action: alloy_relabel.LabelDrop,
		}),
	}
}

// benchTargets builds targets the way service discovery does: grouped by source,
// with each group's shared labels packed once and referenced by every target of
// that group.
func benchTargets(sources, perSource int, generation int) []discovery.Target {
	out := make([]discovery.Target, 0, sources*perSource)
	for s := 0; s < sources; s++ {
		group := commonlabels.LabelSet{
			"__meta_kubernetes_namespace":                         "prod",
			"__meta_kubernetes_service_name":                      commonlabels.LabelValue(fmt.Sprintf("svc-%d", s)),
			"__meta_kubernetes_pod_phase":                         "Running",
			"__meta_kubernetes_pod_annotation_prometheus_io_port": "8080",
			"__meta_kubernetes_pod_label_app":                     commonlabels.LabelValue(fmt.Sprintf("app-%d", s)),
			"__meta_kubernetes_pod_label_team":                    "platform",
			"__meta_kubernetes_pod_label_tier":                    "backend",
			"job":                                                 commonlabels.LabelValue(fmt.Sprintf("job-%d", s)),
		}
		// Pack the group once, then derive each target from it by setting only its
		// own labels. A builder that does not touch group labels keeps the group
		// pointer, so every target of this source shares one copy, which is what
		// toAlloyTargets produces.
		base := discovery.NewTargetFromSpecificAndBaseLabelSet(nil, group)
		for i := 0; i < perSource; i++ {
			tb := discovery.NewTargetBuilderFrom(base)
			tb.Set("__address__", fmt.Sprintf("10.%d.%d.%d:8080", s%256, i%256, generation%256))
			tb.Set("__meta_kubernetes_pod_name", fmt.Sprintf("pod-%d-%d-%d", s, i, generation))
			tb.Set("__meta_kubernetes_pod_ip", fmt.Sprintf("10.%d.%d.%d", s%256, i%256, generation%256))
			tb.Set("__meta_kubernetes_pod_uid", fmt.Sprintf("uid-%d-%d-%d", s, i, generation))
			out = append(out, tb.Target())
		}
	}
	return out
}

func benchComponent(tb testing.TB) *Component {
	tb.Helper()
	return &Component{
		opts: component.Options{
			ID:            "discovery.relabel.bench",
			OnStateChange: func(component.Exports) {},
		},
		cache:              newTargetCache(),
		metrics:            newMetrics(prometheus.NewRegistry()),
		debugDataPublisher: &fakePublisher{},
	}
}

// Benchmark_Cache measures the per-update cost of discovery.relabel for a
// realistic 20k target set, in the shapes an update actually takes.
//
// linux/amd64, Ryzen AI MAX+ 395, 50 sources x 400 targets = 20k targets:
//
//	                           time/op   alloc/op  allocs/op
//	cold                       78.5ms    43.8MB    941k       every target relabeled
//	unchanged snapshot          1.33ms    1.61MB    20.0k     every target a cache hit
//	one of 50 sources changed   3.10ms    2.38MB    38.4k
//	rules changed              82.1ms    45.9MB    1.02M      cache purged, as intended
//
// The 20k allocations left on the fully cached path are the live debugging
// closure built per target in Update, not the cache: with that call removed the
// same benchmark reports 0.99ms and 5 allocations. DebugDataPublisher has no way
// to ask whether anything is listening, so the closure is allocated even when it
// is never called. Fixing that needs a change to the livedebugging service and
// would speed this path up by a further ~40%.
func Benchmark_Cache(b *testing.B) {
	const sources, perSource = 50, 400
	rules := benchRules(b)
	targets := benchTargets(sources, perSource, 0)

	b.Run("cold", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := benchComponent(b)
			if err := c.Update(Arguments{Targets: targets, RelabelConfigs: rules}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unchanged snapshot", func(b *testing.B) {
		c := benchComponent(b)
		args := Arguments{Targets: targets, RelabelConfigs: rules}
		require.NoError(b, c.Update(args)) // warm
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := c.Update(args); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("one source changed", func(b *testing.B) {
		c := benchComponent(b)
		require.NoError(b, c.Update(Arguments{Targets: targets, RelabelConfigs: rules}))

		// Pre-build the per-generation replacements so the benchmark measures the
		// component rather than target construction.
		gens := make([][]discovery.Target, 8)
		for g := range gens {
			next := make([]discovery.Target, len(targets))
			copy(next, targets)
			replacement := benchTargets(1, perSource, g+1)
			copy(next[:perSource], replacement)
			gens[g] = next
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := c.Update(Arguments{Targets: gens[i%len(gens)], RelabelConfigs: rules}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("rules changed", func(b *testing.B) {
		c := benchComponent(b)
		alt := append([]*alloy_relabel.Config(nil), rules...)
		alt = append(alt, benchRule(alloy_relabel.Config{TargetLabel: "extra", Replacement: "a"}))
		alt2 := append([]*alloy_relabel.Config(nil), rules...)
		alt2 = append(alt2, benchRule(alloy_relabel.Config{TargetLabel: "extra", Replacement: "b"}))

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cfg := alt
			if i%2 == 1 {
				cfg = alt2
			}
			if err := c.Update(Arguments{Targets: targets, RelabelConfigs: cfg}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark_Cache_DropHeavy covers the case the cache helps most: rules that drop
// most targets, where the expensive work produces no output at all and would
// otherwise be repeated on every update.
func Benchmark_Cache_DropHeavy(b *testing.B) {
	rules := []*alloy_relabel.Config{
		benchRule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_pod_label_app"},
			Regex:        benchRegexp(b, "app-0"),
			Action:       alloy_relabel.Keep,
		}),
	}
	targets := benchTargets(50, 400, 0)

	b.Run("cold", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := benchComponent(b)
			if err := c.Update(Arguments{Targets: targets, RelabelConfigs: rules}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unchanged snapshot", func(b *testing.B) {
		c := benchComponent(b)
		args := Arguments{Targets: targets, RelabelConfigs: rules}
		require.NoError(b, c.Update(args))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := c.Update(args); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark_Cache_NoRules measures the pass-through path, which bypasses the
// cache entirely.
func Benchmark_Cache_NoRules(b *testing.B) {
	targets := benchTargets(50, 400, 0)
	c := benchComponent(b)
	args := Arguments{Targets: targets}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := c.Update(args); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_CacheKey isolates the cost of the map key itself, which is what makes
// the cache worthwhile: hashing the packed labels must be far cheaper than
// relabeling.
func Benchmark_CacheKey(b *testing.B) {
	targets := benchTargets(1, 1, 0)
	target := targets[0]

	b.Run("CacheKey", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = target.CacheKey()
		}
	})

	b.Run("map lookup", func(b *testing.B) {
		m := map[discovery.TargetCacheKey]*cacheEntry{}
		m[target.CacheKey()] = &cacheEntry{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, ok := m[target.CacheKey()]; !ok {
				b.Fatal("expected a hit")
			}
		}
	})
}
