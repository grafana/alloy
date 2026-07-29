package tailscale_exporter

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"google.golang.org/protobuf/proto"
)

// peerEntry is a cached peer scrape plus the labels to inject on its metrics.
type peerEntry struct {
	raw    []byte
	node   string
	labels map[string]string // e.g. tags, os; the "node" label is added separately
}

// peerMetricsGatherer implements prometheus.Gatherer. It holds a snapshot of
// raw Prometheus text scraped from each peer's Tailscale metrics port, parses
// it on demand, and injects a "node" label (plus any per-peer labels)
// identifying the source peer.
type peerMetricsGatherer struct {
	// cache maps a stable peer identifier to its scraped metrics and labels.
	cache map[string]peerEntry
}

// Gather implements prometheus.Gatherer.
func (g *peerMetricsGatherer) Gather() ([]*dto.MetricFamily, error) {
	var all []*dto.MetricFamily
	var errs prometheus.MultiError
	keys := make([]string, 0, len(g.cache))
	for key := range g.cache {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		entry := g.cache[key]
		families, err := parsePeerMetrics(entry.raw, entry.node, entry.labels)
		if err != nil {
			errs = append(errs, err)
		}
		all = append(all, families...)
	}
	return all, errs.MaybeUnwrap()
}

// parsePeerMetrics parses Prometheus text exposition format from raw and
// injects a "node" label (set to nodeName) plus any extra labels into every
// metric. Extra label keys are applied in sorted order for deterministic output.
func parsePeerMetrics(raw []byte, nodeName string, extra map[string]string) ([]*dto.MetricFamily, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	parsed, err := parser.TextToMetricFamilies(bytes.NewReader(raw))
	var parseErr error
	if err != nil {
		parseErr = fmt.Errorf("parse peer metrics for %q: %w", nodeName, err)
	}

	families := make([]*dto.MetricFamily, 0, len(parsed))
	for _, mf := range parsed {
		// Deep-clone to avoid mutating cached data.
		cloned := proto.Clone(mf).(*dto.MetricFamily)
		for _, m := range cloned.Metric {
			labels := make(map[string]string, len(m.Label)+len(extra)+1)
			for _, pair := range m.Label {
				labels[pair.GetName()] = pair.GetValue()
			}
			labels["node"] = nodeName
			for name, value := range extra {
				if name == "" || value == "" {
					continue
				}
				labels[name] = value
			}

			keys := make([]string, 0, len(labels))
			for name := range labels {
				keys = append(keys, name)
			}
			sort.Strings(keys)

			m.Label = make([]*dto.LabelPair, 0, len(keys))
			for _, name := range keys {
				value := labels[name]
				m.Label = append(m.Label, &dto.LabelPair{Name: &name, Value: &value})
			}
		}
		families = append(families, cloned)
	}
	return families, parseErr
}
