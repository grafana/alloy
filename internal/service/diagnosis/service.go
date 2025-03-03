package diagnosis

import (
	"context"
	"errors"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ServiceName defines the name used for the diagnosis service.
const ServiceName = "diagnosis"

type Options struct {
	Log               log.Logger
	Metrics           prometheus.Registerer
	ClusteringEnabled bool
}

type Service struct {
	logger            log.Logger
	registerer        prometheus.Registerer
	metrics           *metrics
	enabled           bool
	graph             *graph
	clusteringEnabled bool
}

type metrics struct {
	errors   prometheus.Gauge
	warnings prometheus.Gauge
	tips     prometheus.Gauge
}

var _ service.Service = (*Service)(nil)

type Diagnosis interface {
	Diagnosis() ([]insight, error)
}

func New(opts Options) *Service {
	return &Service{
		logger:            opts.Log,
		registerer:        opts.Metrics,
		clusteringEnabled: opts.ClusteringEnabled,
	}
}

type Arguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Enabled: true,
	}
}

// Data implements service.Service.
// It returns the diagnosis for the components to stream.
func (s *Service) Data() any {
	return s
}

// Definition implements service.Service.
func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: Arguments{},
		DependsOn:  []string{},
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

// Run implements service.Service.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	components, err := host.ListComponents("", component.InfoOptions{GetArguments: true})
	if err != nil {
		return err
	}
	s.graph = newGraph(components, s.clusteringEnabled)
	insights := s.applyRules()
	s.report(insights)
	<-ctx.Done()
	return nil
}

// Update implements service.Service.
// TODO: should we unregister metrics when disabled?
func (s *Service) Update(args any) error {
	newArgs := args.(Arguments)
	s.enabled = newArgs.Enabled
	if s.enabled {
		s.registerMetrics()
	}
	return nil
}

func (s *Service) Diagnosis() ([]insight, error) {
	if !s.enabled {
		return nil, errors.New("diagnosis service is not enabled")
	}
	insights := s.applyRules()
	s.report(insights)
	return insights, nil
}

func (s *Service) report(insights []insight) {
	errors, warnings, tips := 0, 0, 0
	for _, insight := range insights {
		switch insight.Level {
		case LevelError:
			level.Error(s.logger).Log("msg", insight.Msg)
			errors++
		case LevelWarning:
			level.Warn(s.logger).Log("msg", insight.Msg)
			warnings++
		case LevelTips:
			level.Info(s.logger).Log("msg", insight.Msg)
			tips++
		}
	}
	s.metrics.errors.Set(float64(errors))
	s.metrics.warnings.Set(float64(warnings))
	s.metrics.tips.Set(float64(tips))
}

func (s *Service) applyRules() []insight {
	insights := make([]insight, 0)
	for _, rule := range rules {
		insights = rule(s.graph, insights)
	}
	return insights
}

func (s *Service) registerMetrics() {
	prom := promauto.With(s.registerer)
	s.metrics = &metrics{
		errors:   prom.NewGauge(prometheus.GaugeOpts{Name: "diagnosis_errors_total"}),
		warnings: prom.NewGauge(prometheus.GaugeOpts{Name: "diagnosis_warnings_total"}),
		tips:     prom.NewGauge(prometheus.GaugeOpts{Name: "diagnosis_tips_total"}),
	}
}
