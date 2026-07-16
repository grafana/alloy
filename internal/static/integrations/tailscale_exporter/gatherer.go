package tailscale_exporter

import (
	"bytes"
	"fmt"
	"sort"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"google.golang.org/protobuf/proto"
)

// peerEntry is a cached peer scrape plus the labels to inject on its metrics.
type peerEntry struct {
	raw    []byte
	labels map[string]string // e.g. tags, os; the "node" label is added separately
}

// peerMetricsGatherer implements prometheus.Gatherer. It holds a snapshot of
// raw Prometheus text scraped from each peer's Tailscale metrics port, parses
// it on demand, and injects a "node" label (plus any per-peer labels)
// identifying the source peer.
type peerMetricsGatherer struct {
	// cache maps peer hostname to its scraped metrics and labels.
	cache map[string]peerEntry
}

// Gather implements prometheus.Gatherer.
func (g *peerMetricsGatherer) Gather() ([]*dto.MetricFamily, error) {
	var all []*dto.MetricFamily
	for node, entry := range g.cache {
		families, err := parsePeerMetrics(entry.raw, node, entry.labels)
		if err != nil {
			// Skip bad peers — don't abort the whole gather.
			continue
		}
		all = append(all, families...)
	}
	return all, nil
}

// parsePeerMetrics parses Prometheus text exposition format from raw and
// injects a "node" label (set to nodeName) plus any extra labels into every
// metric. Extra label keys are applied in sorted order for deterministic output.
func parsePeerMetrics(raw []byte, nodeName string, extra map[string]string) ([]*dto.MetricFamily, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	parsed, err := parser.TextToMetricFamilies(bytes.NewReader(raw))
	if err != nil && len(parsed) == 0 {
		return nil, fmt.Errorf("parse peer metrics for %q: %w", nodeName, err)
	}

	// Assemble the labels to inject: "node" first, then extras in sorted order.
	type kv struct{ name, value string }
	inject := []kv{{"node", nodeName}}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if extra[k] == "" {
			continue // omit empty labels (e.g. untagged nodes)
		}
		inject = append(inject, kv{k, extra[k]})
	}

	families := make([]*dto.MetricFamily, 0, len(parsed))
	for _, mf := range parsed {
		// Deep-clone to avoid mutating cached data.
		cloned := proto.Clone(mf).(*dto.MetricFamily)
		for _, m := range cloned.Metric {
			for _, l := range inject {
				name, value := l.name, l.value
				m.Label = append(m.Label, &dto.LabelPair{Name: &name, Value: &value})
			}
		}
		families = append(families, cloned)
	}
	return families, nil
}
