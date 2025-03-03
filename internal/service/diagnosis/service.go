package diagnosis

import (
	"context"
	"errors"
	"sync"
	"time"

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
	graph             *graph
	recorder          *recorder
	clusteringEnabled bool
	updateCh          chan struct{} // Channel to signal updates

	mu           sync.Mutex // Protects shared fields below
	recordCancel context.CancelFunc
	args         Arguments
}

type metrics struct {
	errors   prometheus.Gauge
	warnings prometheus.Gauge
	tips     prometheus.Gauge
}

var _ service.Service = (*Service)(nil)

type Diagnosis interface {
	Diagnosis(ctx context.Context, host service.Host) ([]insight, error)
}

func New(opts Options) *Service {
	return &Service{
		logger:            opts.Log,
		registerer:        opts.Metrics,
		clusteringEnabled: opts.ClusteringEnabled,
		recorder:          newRecorder(opts.Log),
		graph:             newGraph(opts.ClusteringEnabled),
		updateCh:          make(chan struct{}, 1), // Buffer of 1 to avoid blocking
	}
}

type Arguments struct {
	Enabled  bool          `alloy:"enabled,attr,optional"`
	Window   time.Duration `alloy:"window,attr,optional"`
	Interval time.Duration `alloy:"interval,attr,optional"`
}

func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Enabled: true,
		Window:  time.Minute * 5,
	}
}

func (args *Arguments) Validate() error {
	if args.Window != 0 && args.Interval != 0 && args.Window >= args.Interval {
		return errors.New("window must be less than interval")
	}
	return nil
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

	s.graph.build(components)
	insights := s.applyRules()
	s.report(insights) // report early

	// Process updates or wait for context cancellation
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.updateCh:
			level.Debug(s.logger).Log("msg", "Processing update notification")
			// Refresh components and rebuild graph
			components, err := host.ListComponents("", component.InfoOptions{GetArguments: true})
			if err != nil {
				level.Error(s.logger).Log("msg", "Failed to list components after update", "err", err)
				continue
			}
			s.graph.build(components)
			insights := s.applyRules()
			s.report(insights)

			// Get window value safely
			s.mu.Lock()
			window := s.args.Window
			s.mu.Unlock()

			// Record flow data if window is set
			if window > 0 {
				// Cancel any existing recording (this is an additional safety but it should not be needed)
				s.mu.Lock()
				if s.recordCancel != nil {
					s.recordCancel()
				}
				recordContext, recordCancel := context.WithCancel(ctx)
				s.recordCancel = recordCancel
				s.mu.Unlock()
				level.Info(s.logger).Log("msg", "Start recording data for diagnosis", "window", window)
				flowInsights := s.recorder.record(recordContext, host, window, s.graph)
				level.Info(s.logger).Log("msg", "Finished recording data for diagnosis", "insights", len(flowInsights))
				s.extendReport(flowInsights)
			}
		}
	}
}

// Update implements service.Service.
func (s *Service) Update(args any) error {
	newArgs := args.(Arguments)

	s.mu.Lock()
	s.args = newArgs
	// Cancel any existing recording when updating
	if s.recordCancel != nil {
		s.recordCancel()
		s.recordCancel = nil
	}
	s.mu.Unlock()

	if newArgs.Enabled {
		s.registerMetrics()
	}

	// Send notification through channel (non-blocking)
	select {
	case s.updateCh <- struct{}{}:
	default:
	}

	return nil
}

// Diagnosis implements the Diagnosis interface
func (s *Service) Diagnosis(ctx context.Context, host service.Host) ([]insight, error) {
	s.mu.Lock()
	enabled := s.args.Enabled
	window := s.args.Window
	s.mu.Unlock()

	if !enabled {
		return nil, errors.New("diagnosis service is not enabled")
	}

	insights := s.applyRules()
	flowInsights := s.recorder.record(ctx, host, window, s.graph)
	allInsights := append(insights, flowInsights...)
	s.report(allInsights)
	return allInsights, nil
}

// reportInsights logs insights and updates metrics
// If extend is true, it adds to existing metrics; otherwise it sets them
func (s *Service) reportInsights(insights []insight, extend bool) {
	errors, warnings, tips := 0, 0, 0
	for _, insight := range insights {
		switch insight.Level {
		case LevelError:
			level.Error(s.logger).Log("msg", insight.Msg)
			errors++
		case LevelWarning:
			level.Warn(s.logger).Log("msg", insight.Msg)
			warnings++
		case LevelInfo:
			level.Info(s.logger).Log("msg", insight.Msg)
			tips++
		}
	}

	if extend {
		s.metrics.errors.Add(float64(errors))
		s.metrics.warnings.Add(float64(warnings))
		s.metrics.tips.Add(float64(tips))
	} else {
		s.metrics.errors.Set(float64(errors))
		s.metrics.warnings.Set(float64(warnings))
		s.metrics.tips.Set(float64(tips))
	}
}

func (s *Service) report(insights []insight) {
	s.reportInsights(insights, false)
}

func (s *Service) extendReport(insights []insight) {
	s.reportInsights(insights, true)
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
