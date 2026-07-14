package cluster

import "github.com/prometheus/client_golang/prometheus"

// allocatorMetrics exposes the target-allocator's internal state so a
// mixed/under-converged cluster can be diagnosed from metrics alone:
//
//   - is_leader: which node is the elected allocator leader (and split-brain if
//     more than one reports 1).
//   - registered_targets / weighted_targets (leader only): how many targets the
//     leader knows about, and how many of those have a measured series weight. If
//     weighted_targets stays ~0 while registered_targets is large, weights are not
//     reaching the leader and the assignment stays count-based.
//   - local_series: this node's total measured series across its scrape
//     components. Non-zero proves the local seriesCounter is measuring; compare
//     against the leader's weighted_targets to tell "not measured" from "measured
//     but not delivered".
type allocatorMetrics struct {
	isLeader          prometheus.Gauge
	registeredTargets prometheus.Gauge
	weightedTargets   prometheus.Gauge
	localSeries       prometheus.Gauge
}

func newAllocatorMetrics(clusterName string) *allocatorMetrics {
	labels := prometheus.Labels{"cluster_name": clusterName}
	return &allocatorMetrics{
		isLeader: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "cluster_target_allocator_is_leader",
			Help:        "Reports 1 if this node is the elected target-allocator leader, 0 otherwise.",
			ConstLabels: labels,
		}),
		registeredTargets: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "cluster_target_allocator_registered_targets",
			Help:        "Number of distinct targets the leader has registered for allocation (0 on followers).",
			ConstLabels: labels,
		}),
		weightedTargets: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "cluster_target_allocator_weighted_targets",
			Help:        "Number of registered targets that have a measured series weight on the leader (0 on followers).",
			ConstLabels: labels,
		}),
		localSeries: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "cluster_target_allocator_local_series",
			Help:        "Total series this node has measured across its scrape components and reported for weighting.",
			ConstLabels: labels,
		}),
	}
}

func (m *allocatorMetrics) register(reg prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{m.isLeader, m.registeredTargets, m.weightedTargets, m.localSeries} {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}

// refreshAllocatorMetrics updates the allocator gauges from current state. Safe
// to call on every node; leader-only gauges read 0 on followers.
func (s *Service) refreshAllocatorMetrics() {
	if s.allocatorMetrics == nil {
		return
	}
	if s.alloyCluster.IsAllocatorLeader() {
		s.allocatorMetrics.isLeader.Set(1)
		registered, weighted := s.allocator.stats()
		s.allocatorMetrics.registeredTargets.Set(float64(registered))
		s.allocatorMetrics.weightedTargets.Set(float64(weighted))
	} else {
		s.allocatorMetrics.isLeader.Set(0)
		s.allocatorMetrics.registeredTargets.Set(0)
		s.allocatorMetrics.weightedTargets.Set(0)
	}
	s.allocatorMetrics.localSeries.Set(float64(s.alloyCluster.localSeriesSum()))
}
