package scrape

import (
	"context"
	"strconv"
	"sync"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/discovery"
)

// seriesCounter wraps a storage.Appendable and records, per scrape target, how
// many series that target produced on its most recent scrape. It is used for
// series-aware (weighted) clustering in allocator mode: the measured counts are
// reported to the target-allocator leader, which weights the assignment so the
// owner of a high-series "whale" target carries fewer other targets.
//
// Attribution works because Prometheus puts the scrape Target in the appender
// context (scrape.ContextWithTarget); the target carries its precomputed
// clustering key as the ClusteringKeyMetaLabel meta-label (stamped by the scrape
// component). When no key is stamped (clustering off, or a non-clustered target),
// the wrapper is a transparent pass-through that counts nothing.
type seriesCounter struct {
	next storage.Appendable

	mut    sync.Mutex
	counts map[uint64]uint64 // target clustering key -> series on last scrape
}

func newSeriesCounter(next storage.Appendable) *seriesCounter {
	return &seriesCounter{next: next, counts: make(map[uint64]uint64)}
}

// Appender implements storage.Appendable.
func (s *seriesCounter) Appender(ctx context.Context) storage.Appender {
	ca := &countingAppender{Appender: s.next.Appender(ctx), parent: s}

	// Resolve the target's clustering key from the appender context, if present.
	if t, ok := scrape.TargetFromContext(ctx); ok && t != nil {
		lb := labels.NewBuilder(labels.EmptyLabels())
		if raw := t.DiscoveredLabels(lb).Get(discovery.ClusteringKeyMetaLabel); raw != "" {
			if k, err := strconv.ParseUint(raw, 10, 64); err == nil {
				ca.key = k
				ca.hasKey = true
			}
		}
	}
	return ca
}

// record stores the latest series count for a target key.
func (s *seriesCounter) record(key, n uint64) {
	s.mut.Lock()
	s.counts[key] = n
	s.mut.Unlock()
}

// Weights returns a copy of the most recent per-target series counts, keyed by
// clustering key. Suitable to hand to cluster.ReportWeights.
func (s *seriesCounter) Weights() map[uint64]uint64 {
	s.mut.Lock()
	defer s.mut.Unlock()
	out := make(map[uint64]uint64, len(s.counts))
	for k, v := range s.counts {
		out[k] = v
	}
	return out
}

// countingAppender embeds the real per-scrape appender (so every method it
// doesn't override is promoted unchanged) and counts the series appended for one
// scrape, recording the total into the parent on Commit.
type countingAppender struct {
	storage.Appender
	parent *seriesCounter
	key    uint64
	hasKey bool
	n      uint64
}

func (a *countingAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.n++
	return a.Appender.Append(ref, l, t, v)
}

func (a *countingAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	a.n++
	return a.Appender.AppendHistogram(ref, l, t, h, fh)
}

func (a *countingAppender) Commit() error {
	if a.hasKey {
		a.parent.record(a.key, a.n)
	}
	return a.Appender.Commit()
}
