// This file is in the external test package so that it can import
// common/relabel alongside discovery and exercise the real relabel rule engine
// against discovery.TargetBuilder. The in-package benchmarks cannot do this,
// which is why Benchmark_Targets_TypicalPipeline only approximates the relabel
// step.
package discovery_test

import (
	"fmt"
	"testing"

	commonlabels "github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
)

func mustRegexp(tb testing.TB, s string) alloy_relabel.Regexp {
	tb.Helper()
	// The Regexp constructor is unexported, but Regexp implements
	// encoding.TextUnmarshaler, which is how relabel rules are decoded.
	var re alloy_relabel.Regexp
	require.NoError(tb, re.UnmarshalText([]byte(s)))
	return re
}

// kubernetesRules mirrors the "kubernetes" case of
// common/relabel.BenchmarkRelabel: a realistic pod-scraping rule set covering
// replace, labelmap, hashmod, keep, drop and labeldrop.
func kubernetesRules(tb testing.TB) []*alloy_relabel.Config {
	tb.Helper()

	rule := func(c alloy_relabel.Config) *alloy_relabel.Config {
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

	return []*alloy_relabel.Config{
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_pod_annotation_prometheus_io_scrape"},
			Regex:        mustRegexp(tb, "true"),
			Action:       alloy_relabel.Keep,
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__address__", "__meta_kubernetes_pod_annotation_prometheus_io_port"},
			Regex:        mustRegexp(tb, `(.+?)(\:\d+)?;(\d+)`),
			TargetLabel:  "__address__",
			Replacement:  "$1:$3",
			Action:       alloy_relabel.Replace,
		}),
		rule(alloy_relabel.Config{
			Regex:       mustRegexp(tb, "__meta_kubernetes_pod_label_prometheus_io_label_(.+)"),
			Action:      alloy_relabel.LabelMap,
			Replacement: "$1",
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_namespace", "__meta_kubernetes_pod_label_name"},
			Separator:    "/",
			TargetLabel:  "job",
			Replacement:  "$1",
			Action:       alloy_relabel.Replace,
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_namespace"},
			TargetLabel:  "namespace",
			Action:       alloy_relabel.Replace,
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_pod_name"},
			TargetLabel:  "pod",
			Action:       alloy_relabel.Replace,
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
			TargetLabel:  "container",
			Action:       alloy_relabel.Replace,
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{
				"__meta_kubernetes_pod_name",
				"__meta_kubernetes_pod_container_name",
				"__meta_kubernetes_pod_container_port_name",
			},
			Separator:   ":",
			TargetLabel: "instance",
			Action:      alloy_relabel.Replace,
		}),
		rule(alloy_relabel.Config{
			TargetLabel: "cluster",
			Replacement: "dev-us-central-0",
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__meta_kubernetes_namespace"},
			Regex:        mustRegexp(tb, "hosted-grafana"),
			Action:       alloy_relabel.Drop,
		}),
		rule(alloy_relabel.Config{
			SourceLabels: []string{"__address__"},
			TargetLabel:  "__tmp_hash",
			Modulus:      3,
			Action:       alloy_relabel.HashMod,
		}),
		rule(alloy_relabel.Config{
			Regex:  mustRegexp(tb, "__tmp_hash"),
			Action: alloy_relabel.LabelDrop,
		}),
	}
}

// kubernetesTarget returns the group labels shared by every pod of a service and
// the labels specific to one pod, mirroring how Kubernetes SD populates a
// targetgroup.Group.
func kubernetesTarget(ind int) (group, own commonlabels.LabelSet) {
	group = commonlabels.LabelSet{
		"__meta_kubernetes_namespace":                                  "loki-boltdb-shipper",
		"__meta_kubernetes_pod_annotation_prometheus_io_scrape":        "true",
		"__meta_kubernetes_pod_annotation_prometheus_io_port":          "80",
		"__meta_kubernetes_pod_annotationpresent_prometheus_io_scrape": "true",
		"__meta_kubernetes_pod_container_init":                         "false",
		"__meta_kubernetes_pod_container_name":                         "promtail",
		"__meta_kubernetes_pod_container_port_name":                    "http-metrics",
		"__meta_kubernetes_pod_container_port_number":                  "80",
		"__meta_kubernetes_pod_container_port_protocol":                "TCP",
		"__meta_kubernetes_pod_controller_kind":                        "DaemonSet",
		"__meta_kubernetes_pod_controller_name":                        "promtail-loki-boltdb-shipper",
		"__meta_kubernetes_pod_label_name":                             "promtail-loki-boltdb-shipper",
		"__meta_kubernetes_pod_label_prometheus_io_label_team":         "logs",
		"__meta_kubernetes_pod_labelpresent_name":                      "true",
		"__meta_kubernetes_pod_phase":                                  "Running",
		"__meta_kubernetes_pod_ready":                                  "true",
		"__metrics_path__":                                             "/metrics",
		"__scheme__":                                                   "http",
		"__scrape_interval__":                                          "15s",
		"__scrape_timeout__":                                           "10s",
		"job":                                                          "kubernetes-pods",
	}
	own = commonlabels.LabelSet{
		"__address__":                     commonlabels.LabelValue(fmt.Sprintf("10.132.183.%d:80", ind%256)),
		"__meta_kubernetes_pod_host_ip":   commonlabels.LabelValue(fmt.Sprintf("10.128.0.%d", ind%256)),
		"__meta_kubernetes_pod_ip":        commonlabels.LabelValue(fmt.Sprintf("10.132.183.%d", ind%256)),
		"__meta_kubernetes_pod_name":      commonlabels.LabelValue(fmt.Sprintf("promtail-loki-boltdb-shipper-%d", ind)),
		"__meta_kubernetes_pod_node_name": commonlabels.LabelValue(fmt.Sprintf("gke-dev-us-central-0-main-n2s8-2-%d", ind)),
		"__meta_kubernetes_pod_uid":       commonlabels.LabelValue(fmt.Sprintf("4c586419-7f6c-448d-aeec-ca4fa5b0%04d", ind)),
	}
	return group, own
}

// Benchmark_Relabel_TargetBuilder measures the real relabeling path that
// discovery.relabel and the database_observability components take: build a
// TargetBuilder from a Target, run the Alloy relabel rule engine over it, and
// materialise the result. This is the main workload the packed label layout is
// meant to improve, and it was previously unmeasured.
func Benchmark_Relabel_TargetBuilder(b *testing.B) {
	rules := kubernetesRules(b)
	const targetsCount = 2_000

	// Group labels are shared across the whole set, as they would be for targets
	// coming from a single Kubernetes SD target group.
	sharedGroup, _ := kubernetesTarget(0)
	targets := make([]discovery.Target, 0, targetsCount)
	for i := 0; i < targetsCount; i++ {
		_, own := kubernetesTarget(i)
		targets = append(targets, discovery.NewTargetFromSpecificAndBaseLabelSet(own, sharedGroup))
	}

	// Sanity check that the rules actually keep targets and do real work, so the
	// benchmark cannot silently degrade into a no-op.
	{
		tb := discovery.NewTargetBuilderFrom(targets[0])
		require.True(b, alloy_relabel.ProcessBuilder(tb, rules...))
		result := tb.Target()
		_, hasTmp := result.Get("__tmp_hash")
		require.False(b, hasTmp, "labeldrop should have removed __tmp_hash")
		cluster, ok := result.Get("cluster")
		require.True(b, ok)
		require.Equal(b, "dev-us-central-0", cluster)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out := make([]discovery.Target, 0, len(targets))
		for _, t := range targets {
			tb := discovery.NewTargetBuilderFrom(t)
			if alloy_relabel.ProcessBuilder(tb, rules...) {
				out = append(out, tb.Target())
			}
		}
		if len(out) == 0 {
			b.Fatal("expected at least some targets to be kept")
		}
	}
}

// Benchmark_Relabel_NoChanges measures the copy-on-write fast path: rules that
// inspect labels but change nothing must not re-encode either label buffer.
func Benchmark_Relabel_NoChanges(b *testing.B) {
	rules := []*alloy_relabel.Config{
		func() *alloy_relabel.Config {
			c := alloy_relabel.DefaultRelabelConfig
			c.SourceLabels = []string{"__meta_kubernetes_pod_annotation_prometheus_io_scrape"}
			c.Regex = mustRegexp(b, "true")
			c.Action = alloy_relabel.Keep
			return &c
		}(),
	}

	sharedGroup, own := kubernetesTarget(0)
	target := discovery.NewTargetFromSpecificAndBaseLabelSet(own, sharedGroup)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tb := discovery.NewTargetBuilderFrom(target)
		if !alloy_relabel.ProcessBuilder(tb, rules...) {
			b.Fatal("target should be kept")
		}
		_ = tb.Target()
	}
}

// Benchmark_Target_EqualsTarget measures the comparison the runtime performs on
// every component export to decide whether downstream components need to be
// re-evaluated.
func Benchmark_Target_EqualsTarget(b *testing.B) {
	sharedGroup, own := kubernetesTarget(0)
	left := discovery.NewTargetFromSpecificAndBaseLabelSet(own, sharedGroup)

	b.Run("unchanged", func(b *testing.B) {
		// Same group pointer and same own buffer, as produced by a relabel step
		// that changed nothing. This is the overwhelmingly common case.
		right := discovery.NewTargetBuilderFrom(left).Target()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if !left.EqualsTarget(&right) {
				b.Fatal("targets should be equal")
			}
		}
	})

	b.Run("equal but rebuilt", func(b *testing.B) {
		// Same labels, independently constructed, so the fast path does not apply.
		right := discovery.NewTargetFromSpecificAndBaseLabelSet(own, sharedGroup)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if !left.EqualsTarget(&right) {
				b.Fatal("targets should be equal")
			}
		}
	})

	b.Run("different", func(b *testing.B) {
		_, otherOwn := kubernetesTarget(1)
		right := discovery.NewTargetFromSpecificAndBaseLabelSet(otherOwn, sharedGroup)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if left.EqualsTarget(&right) {
				b.Fatal("targets should differ")
			}
		}
	})
}

// Benchmark_Target_Get measures a single label lookup, which the packed layout
// makes an O(n) scan instead of a map lookup. Targets in this benchmark carry a
// realistic number of labels.
func Benchmark_Target_Get(b *testing.B) {
	sharedGroup, own := kubernetesTarget(0)
	target := discovery.NewTargetFromSpecificAndBaseLabelSet(own, sharedGroup)

	// Cover a label at the start of the buffer, one in the middle, one in the
	// group rather than own, and one that is absent.
	for _, name := range []string{"__address__", "__meta_kubernetes_pod_name", "job", "does_not_exist"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = target.Get(name)
			}
		})
	}
}
