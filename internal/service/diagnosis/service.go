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
	recorder          *recorder
	clusteringEnabled bool
	metricsRegistered bool
	updateCh          chan struct{} // Channel to signal updates

	mu           sync.Mutex // Protects shared fields below
	recordCancel context.CancelFunc
	args         Arguments
	ticker       *time.Ticker // Ticker for periodic diagnoses
}

type metrics struct {
	errors   prometheus.Gauge
	warnings prometheus.Gauge
	tips     prometheus.Gauge
}

var _ service.Service = (*Service)(nil)

type Diagnosis interface {
	Diagnosis(ctx context.Context, host service.Host, window time.Duration) ([]insight, error)
}

func New(opts Options) *Service {
	return &Service{
		logger:            opts.Log,
		registerer:        opts.Metrics,
		clusteringEnabled: opts.ClusteringEnabled,
		recorder:          newRecorder(opts.Log),
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
	s.mu.Lock()
	if s.args.Interval > 0 {
		s.ticker = time.NewTicker(s.args.Interval)
	}
	s.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			if s.ticker != nil {
				s.ticker.Stop()
			}
			s.mu.Unlock()
			return nil
		case <-s.updateCh:
			s.diagnose(ctx, host)
		case <-s.tickerChan():
			s.diagnose(ctx, host)
		}
	}
}

func (s *Service) tickerChan() <-chan time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ticker != nil {
		return s.ticker.C
	}
	return nil // Returning nil is safe in select; it will be ignored.
}

func (s *Service) diagnose(ctx context.Context, host service.Host) {
	graphs, err := s.buildGraphs(host)
	if err != nil {
		level.Error(s.logger).Log("msg", "Failed to build graphs for diagnosis", "err", err)
		return
	}

	insights := make([]insight, 0)
	insights = s.applyRules(graphs, insights)
	s.report(insights) // report early

	s.mu.Lock()
	window := s.args.Window
	s.mu.Unlock()

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
		flowInsights, err := s.recorder.record(recordContext, host, window, graphs)
		if err != nil {
			level.Info(s.logger).Log("msg", "Recording data for diagnosis did not work", "reason", err)
			return
		}
		level.Info(s.logger).Log("msg", "Finished recording data for diagnosis", "insights", len(flowInsights))
		s.extendReport(flowInsights)
	}
}

// Update implements service.Service.
func (s *Service) Update(args any) error {
	newArgs := args.(Arguments)

	s.mu.Lock()
	s.args = newArgs
	if s.recordCancel != nil {
		s.recordCancel()
		s.recordCancel = nil
	}
	s.mu.Unlock()

	if newArgs.Enabled && !s.metricsRegistered {
		s.registerMetrics()
		s.metricsRegistered = true
	} else if !newArgs.Enabled && s.metricsRegistered {
		s.metricsRegistered = false
		s.registerer.Unregister(s.metrics.errors)
		s.registerer.Unregister(s.metrics.warnings)
		s.registerer.Unregister(s.metrics.tips)
	}

	select {
	case s.updateCh <- struct{}{}:
	default:
	}

	return nil
}

func (s *Service) buildGraphs(host service.Host) ([]*graph, error) {
	modules := host.ListModules()
	modules = append(modules, "") // Add root module
	graphs := make([]*graph, len(modules))
	for i, module := range modules {
		components, err := host.ListComponents(module, component.InfoOptions{GetArguments: true})
		if err != nil {
			return nil, err
		}
		graphs[i] = newGraph(module, s.clusteringEnabled)
		graphs[i].build(components)
	}
	return graphs, nil
}

// Diagnosis implements the Diagnosis interface
func (s *Service) Diagnosis(ctx context.Context, host service.Host, window time.Duration) ([]insight, error) {
	s.mu.Lock()
	enabled := s.args.Enabled
	s.mu.Unlock()

	if !enabled {
		return nil, errors.New("diagnosis service is not enabled")
	}

	graphs, err := s.buildGraphs(host)
	if err != nil {
		return nil, err
	}

	insights := make([]insight, 0)
	insights = s.applyRules(graphs, insights)
	if window > 0 {
		flowInsights, err := s.recorder.record(ctx, host, window, graphs)
		if err != nil {
			return nil, err
		}
		allInsights := append(insights, flowInsights...)
		s.reportWithoutMetrics(allInsights)
		return allInsights, nil
	}
	s.reportWithoutMetrics(insights)
	return insights, nil
}

// reportInsights logs insights and updates metrics
// If extend is true, it adds to existing metrics; otherwise it sets them
func (s *Service) reportInsights(insights []insight, extend bool, withMetrics bool) {
	errors, warnings, tips := 0, 0, 0
	for _, insight := range insights {
		switch insight.Level {
		case LevelError:
			level.Error(s.logger).Log("msg", insight.Msg, "module", insight.Module)
			errors++
		case LevelWarning:
			level.Warn(s.logger).Log("msg", insight.Msg, "module", insight.Module)
			warnings++
		case LevelInfo:
			level.Info(s.logger).Log("msg", insight.Msg, "module", insight.Module)
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
	s.reportInsights(insights, false, true)
}

func (s *Service) reportWithoutMetrics(insights []insight) {
	s.reportInsights(insights, false, false)
}

func (s *Service) extendReport(insights []insight) {
	s.reportInsights(insights, true, true)
}

func (s *Service) applyRules(graphs []*graph, insights []insight) []insight {
	for _, graph := range graphs {
		for _, rule := range rules {
			insights = rule(graph, insights)
		}
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
