package tailscale_exporter

import (
	"bytes"
	"fmt"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"google.golang.org/protobuf/proto"
)

// peerMetricsGatherer implements prometheus.Gatherer. It holds a snapshot of
// raw Prometheus text scraped from each peer's Tailscale metrics port, parses
// it on demand, and injects a "node" label identifying the source peer.
type peerMetricsGatherer struct {
	// cache maps peer hostname to raw Prometheus exposition text.
	cache map[string][]byte
}

// Gather implements prometheus.Gatherer.
func (g *peerMetricsGatherer) Gather() ([]*dto.MetricFamily, error) {
	var all []*dto.MetricFamily
	for node, raw := range g.cache {
		families, err := parsePeerMetrics(raw, node)
		if err != nil {
			// Skip bad peers — don't abort the whole gather.
			continue
		}
		all = append(all, families...)
	}
	return all, nil
}

// parsePeerMetrics parses Prometheus text exposition format from raw and
// returns metric families with a "node" label set to nodeName injected into
// every metric.
func parsePeerMetrics(raw []byte, nodeName string) ([]*dto.MetricFamily, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	parsed, err := parser.TextToMetricFamilies(bytes.NewReader(raw))
	if err != nil && len(parsed) == 0 {
		return nil, fmt.Errorf("parse peer metrics for %q: %w", nodeName, err)
	}

	nodeLabel := "node"
	nodeValue := nodeName

	families := make([]*dto.MetricFamily, 0, len(parsed))
	for _, mf := range parsed {
		// Deep-clone to avoid mutating cached data.
		cloned := proto.Clone(mf).(*dto.MetricFamily)
		for _, m := range cloned.Metric {
			m.Label = append(m.Label, &dto.LabelPair{
				Name:  &nodeLabel,
				Value: &nodeValue,
			})
		}
		families = append(families, cloned)
	}
	return families, nil
}
