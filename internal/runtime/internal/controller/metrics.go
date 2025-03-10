package controller

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// controllerMetrics contains the metrics for components controller
type controllerMetrics struct {
	controllerEvaluation        prometheus.Gauge
	componentEvaluationTime     prometheus.Histogram
	dependenciesWaitTime        prometheus.Histogram
	evaluationQueueSize         prometheus.Gauge
	slowComponentThreshold      time.Duration
	slowComponentEvaluationTime *prometheus.CounterVec
}

// newControllerMetrics inits the metrics for the components controller
func newControllerMetrics(parent, id string) *controllerMetrics {
	cm := &controllerMetrics{
		slowComponentThreshold: 1 * time.Minute,
	}

	// The evaluation time becomes particularly problematic in the range of 30s+, so add more buckets
	// that can help spot issues in that range.
	// Use the following buckets: 5ms, 25ms, 100ms, 500ms, 1s, 5s, 10s, 30s, 1m, 2m, 5m, 10m
	evaluationTimesBuckets := []float64{.005, .025, .1, .5, 1, 5, 10, 30, 60, 120, 300, 600}

	cm.controllerEvaluation = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "alloy_component_controller_evaluating",
		Help:        "Tracks if the controller is currently in the middle of a graph evaluation",
		ConstLabels: map[string]string{"controller_path": parent, "controller_id": id},
	})

	cm.componentEvaluationTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:                            "alloy_component_evaluation_seconds",
			Help:                            "Time spent performing component evaluation",
			ConstLabels:                     map[string]string{"controller_path": parent, "controller_id": id},
			Buckets:                         evaluationTimesBuckets,
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
	)
	cm.dependenciesWaitTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:                            "alloy_component_dependencies_wait_seconds",
			Help:                            "Time spent by components waiting to be evaluated after their dependency is updated.",
			ConstLabels:                     map[string]string{"controller_path": parent, "controller_id": id},
			Buckets:                         evaluationTimesBuckets,
			NativeHistogramBucketFactor:     1.1,
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
	)

	cm.evaluationQueueSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "alloy_component_evaluation_queue_size",
		Help:        "Tracks the number of components waiting to be evaluated in the worker pool",
		ConstLabels: map[string]string{"controller_path": parent, "controller_id": id},
	})

	cm.slowComponentEvaluationTime = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "alloy_component_evaluation_slow_seconds",
		Help:        fmt.Sprintf("Number of seconds spent evaluating components that take longer than %v to evaluate", cm.slowComponentThreshold),
		ConstLabels: map[string]string{"controller_path": parent, "controller_id": id},
	}, []string{"component_id"})

	return cm
}

func (cm *controllerMetrics) onComponentEvaluationDone(name string, duration time.Duration) {
	cm.componentEvaluationTime.Observe(duration.Seconds())
	if duration >= cm.slowComponentThreshold {
		cm.slowComponentEvaluationTime.WithLabelValues(name).Add(duration.Seconds())
	}
}

func (cm *controllerMetrics) Collect(ch chan<- prometheus.Metric) {
	cm.componentEvaluationTime.Collect(ch)
	cm.controllerEvaluation.Collect(ch)
	cm.dependenciesWaitTime.Collect(ch)
	cm.evaluationQueueSize.Collect(ch)
	cm.slowComponentEvaluationTime.Collect(ch)
}

func (cm *controllerMetrics) Describe(ch chan<- *prometheus.Desc) {
	cm.componentEvaluationTime.Describe(ch)
	cm.controllerEvaluation.Describe(ch)
	cm.dependenciesWaitTime.Describe(ch)
	cm.evaluationQueueSize.Describe(ch)
	cm.slowComponentEvaluationTime.Describe(ch)
}

type controllerCollector struct {
	l                      *Loader
	runningComponentsTotal *prometheus.Desc
	collectorHealth        *prometheus.Desc
}

func newControllerCollector(l *Loader, parent, id string) *controllerCollector {
	return &controllerCollector{
		l: l,
		collectorHealth: prometheus.NewDesc(
			"alloy_component_controller_health",
			"Component health",
			[]string{"health_type", "component_name"},
			map[string]string{"controller_path": parent, "controller_id": id},
		),
		runningComponentsTotal: prometheus.NewDesc(
			"alloy_component_controller_running_components",
			"Total number of running components.", nil,
			map[string]string{"controller_path": parent, "controller_id": id},
		),
	}
}

type componentInfo struct {
	Name   string
	Health string
}

var HEALTH_STATES = []string{"healthy", "unhealthy"}

func (cc *controllerCollector) Collect(ch chan<- prometheus.Metric) {
	componentsByHealth := []componentInfo{}
	var componentCount int
	for _, component := range cc.l.Components() {
		componentCount++
		health := component.CurrentHealth().Health.String()
		componentName := component.ComponentName()
		componentInfo := componentInfo{Name: componentName, Health: health}
		componentsByHealth = append(componentsByHealth, componentInfo)

		if builtinComponent, ok := component.(*BuiltinComponentNode); ok {
			builtinComponent.registry.Collect(ch)
		}

	}

	for _, im := range cc.l.Imports() {
		componentCount++
		health := im.CurrentHealth().Health.String()
		componentName := im.componentName
		componentInfo := componentInfo{Name: componentName, Health: health}
		componentsByHealth = append(componentsByHealth, componentInfo)

		im.registry.Collect(ch)

	}

	ch <- prometheus.MustNewConstMetric(cc.runningComponentsTotal, prometheus.GaugeValue, float64(componentCount))

	var healthValue float64
	for _, component := range componentsByHealth {
		for _, healthState := range HEALTH_STATES {
			if component.Health == healthState {
				healthValue = 1
			} else {
				healthValue = 0
			}
			// Create a metric for each component's health state
			ch <- prometheus.MustNewConstMetric(
				cc.collectorHealth,
				prometheus.GaugeValue,
				healthValue,
				healthState,
				component.Name,
			)
		}
	}
}

func (cc *controllerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cc.runningComponentsTotal
	ch <- cc.collectorHealth
}
