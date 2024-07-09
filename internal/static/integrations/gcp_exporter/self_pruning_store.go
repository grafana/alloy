package gcp_exporter

import (
	"context"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus-community/stackdriver_exporter/collectors"
	"github.com/tilinna/clock"
	"go.uber.org/atomic"
	"google.golang.org/api/monitoring/v3"
)

// Five minutes in seconds for Unix comparisons
const FiveMinutePruneWindow int64 = 5 * 60

type CounterOrHistogramStore[T CounterOrHistogram] interface {
	Increment(metricDescriptor *monitoring.MetricDescriptor, currentValue *T)
	ListMetrics(metricDescriptorName string) []*T
}

type CounterOrHistogram interface {
	collectors.ConstMetric | collectors.HistogramMetric
}

type SelfPruningDeltaStore[T CounterOrHistogram] struct {
	wrapping                     CounterOrHistogramStore[T]
	mux                          sync.Mutex
	trackedMetricDescriptorNames map[string]struct{}
	lastListOperationTime        atomic.Int64
	logger                       log.Logger
}

// NewSelfPruningDeltaStore provides a configured instance of the SelfPruningDeltaStore which wraps an existing delta
// store implementation with support for pruning the store when it's not being used as a part of normal operations.
//
// The GCP exporter naturally prunes the store over time during normal operations. If the exporter is being used in
// clustering mode and does not have active GCP targets this will not happen. If it later has targets assigned any
// old counter values will be used potentially causing invalid rate and increase calculations.
//
// This is a short term fix until clustering aware components are completed. This will ensure the in-memory counters
// are pruned when an exporter instance no longer has targets assigned.
func NewSelfPruningDeltaStore[T CounterOrHistogram](l log.Logger, wrapping CounterOrHistogramStore[T]) *SelfPruningDeltaStore[T] {
	return &SelfPruningDeltaStore[T]{
		logger:                       l,
		wrapping:                     wrapping,
		trackedMetricDescriptorNames: map[string]struct{}{},
	}
}

// Increment delegates to the wrapped store
// We do not track metric descriptors from here to avoid more locking in a high throughput function
func (s *SelfPruningDeltaStore[T]) Increment(metricDescriptor *monitoring.MetricDescriptor, currentValue *T) {
	s.wrapping.Increment(metricDescriptor, currentValue)
}

// ListMetrics delegates to the wrapped store and updates tracking for the metricDescriptorName based on the results
func (s *SelfPruningDeltaStore[T]) ListMetrics(metricDescriptorName string) []*T {
	s.lastListOperationTime.Store(time.Now().Unix())
	result := s.wrapping.ListMetrics(metricDescriptorName)

	s.mux.Lock()
	defer s.mux.Unlock()
	// We only care to add to tracking when the descriptor has results and remove it when it no longer has results
	_, ok := s.trackedMetricDescriptorNames[metricDescriptorName]
	if !ok && len(result) > 0 {
		s.trackedMetricDescriptorNames[metricDescriptorName] = struct{}{}
	} else if ok && len(result) == 0 {
		delete(s.trackedMetricDescriptorNames, metricDescriptorName)
	}

	return result
}

func (s *SelfPruningDeltaStore[T]) Prune(ctx context.Context) {
	now := clock.Now(ctx).Unix()
	if s.shouldPrune(now) {
		level.Debug(s.logger).Log("msg", "Pruning window breached starting prune")
		s.mux.Lock()
		defer s.mux.Unlock()
		for descriptor := range s.trackedMetricDescriptorNames {
			// Early eject if ListMetrics is being called again
			if !s.shouldPrune(now) {
				level.Debug(s.logger).Log("msg", "Store no longer needs pruned stopping")
				break
			}
			// Calling ListMetrics has a side effect of pruning any data outside a configured TTL we want to make sure
			// this will always continue to happen
			result := s.wrapping.ListMetrics(descriptor)
			if len(result) == 0 {
				delete(s.trackedMetricDescriptorNames, descriptor)
			}
			level.Debug(s.logger).Log("msg", "Pruning metric descriptor", "metric_descriptor", descriptor, "metrics_remaining", len(result))
		}
		level.Debug(s.logger).Log("msg", "Pruning finished")
	}
}

func (s *SelfPruningDeltaStore[T]) shouldPrune(now int64) bool {
	return (now - s.lastListOperationTime.Load()) > FiveMinutePruneWindow
}
