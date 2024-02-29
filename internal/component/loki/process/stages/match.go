package stages

import (
	"errors"
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/loki/clients/pkg/logentry/logql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

// Configuration errors.
var (
	ErrEmptyMatchStageConfig = errors.New("match stage config cannot be empty")
	ErrPipelineNameRequired  = errors.New("match stage pipeline name can be omitted but cannot be an empty string")
	ErrSelectorRequired      = errors.New("selector statement required for match stage")
	ErrMatchRequiresStages   = errors.New("match stage requires at least one additional stage to be defined in '- stages'")
	ErrSelectorSyntax        = errors.New("invalid selector syntax for match stage")
	ErrStagesWithDropLine    = errors.New("match stage configured to drop entries cannot contains stages")
	ErrUnknownMatchAction    = errors.New("match stage action should be 'keep' or 'drop'")

	MatchActionKeep = "keep"
	MatchActionDrop = "drop"
)

// MatchConfig contains the configuration for a matcherStage
type MatchConfig struct {
	Selector     string        `river:"selector,attr"`
	Stages       []StageConfig `river:"stage,enum,optional"`
	Action       string        `river:"action,attr,optional"`
	PipelineName string        `river:"pipeline_name,attr,optional"`
	DropReason   string        `river:"drop_counter_reason,attr,optional"`
}

// validateMatcherConfig validates the MatcherConfig for the matcherStage
func validateMatcherConfig(cfg *MatchConfig) (logql.Expr, error) {
	if cfg.Selector == "" {
		return nil, ErrSelectorRequired
	}
	switch cfg.Action {
	case MatchActionKeep, MatchActionDrop:
	case "":
		cfg.Action = MatchActionKeep
	default:
		return nil, ErrUnknownMatchAction
	}

	if cfg.Action == MatchActionKeep && (cfg.Stages == nil || len(cfg.Stages) == 0) {
		return nil, ErrMatchRequiresStages
	}
	if cfg.Action == MatchActionDrop && (cfg.Stages != nil && len(cfg.Stages) != 0) {
		return nil, ErrStagesWithDropLine
	}

	selector, err := logql.ParseExpr(cfg.Selector)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", ErrSelectorSyntax, err)
	}
	return selector, nil
}

// newMatcherStage creates a new matcherStage from config
func newMatcherStage(logger log.Logger, jobName *string, config MatchConfig, registerer prometheus.Registerer) (Stage, error) {
	selector, err := validateMatcherConfig(&config)
	if err != nil {
		return nil, err
	}

	var nPtr *string
	if config.PipelineName != "" && jobName != nil {
		name := *jobName + "_" + config.PipelineName
		nPtr = &name
	}

	var pl *Pipeline
	if config.Action == MatchActionKeep {
		var err error
		pl, err = NewPipeline(logger, config.Stages, nPtr, registerer)
		if err != nil {
			return nil, fmt.Errorf("%v: %w", err, fmt.Errorf("match stage failed to create pipeline from config: %v", config))
		}
	}

	filter, err := selector.Filter()
	if err != nil {
		return nil, fmt.Errorf("%v: %w", "error parsing pipeline", err)
	}

	dropReason := "match_stage"
	if config.DropReason != "" {
		dropReason = config.DropReason
	}

	return &matcherStage{
		dropReason: dropReason,
		dropCount:  getDropCountMetric(registerer),
		matchers:   selector.Matchers(),
		stage:      pl,
		action:     config.Action,
		filter:     filter,
	}, nil
}

func getDropCountMetric(registerer prometheus.Registerer) *prometheus.CounterVec {
	dropCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_process_dropped_lines_total",
		Help: "A count of all log lines dropped as a result of a pipeline stage",
	}, []string{"reason"})
	err := registerer.Register(dropCount)
	if err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			dropCount = existing.ExistingCollector.(*prometheus.CounterVec)
		} else {
			// Same behavior as MustRegister if the error is not for AlreadyRegistered
			panic(err)
		}
	}
	return dropCount
}

// matcherStage applies Label matchers to determine if the include stages should be run
type matcherStage struct {
	dropReason string
	dropCount  *prometheus.CounterVec
	matchers   []*labels.Matcher
	filter     logql.Filter
	stage      Stage
	action     string
}

func (m *matcherStage) Run(in chan Entry) chan Entry {
	switch m.action {
	case MatchActionDrop:
		return m.runDrop(in)
	case MatchActionKeep:
		return m.runKeep(in)
	}
	panic("unexpected action")
}

func (m *matcherStage) runKeep(in chan Entry) chan Entry {
	next := make(chan Entry)
	out := make(chan Entry)
	outNext := m.stage.Run(next)
	go func() {
		defer close(out)
		for e := range outNext {
			out <- e
		}
	}()
	go func() {
		defer close(next)
		for e := range in {
			e, ok := m.processLogQL(e)
			if !ok {
				out <- e
				continue
			}
			next <- e
		}
	}()
	return out
}

func (m *matcherStage) runDrop(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			if e, ok := m.processLogQL(e); !ok {
				out <- e
				continue
			}
			m.dropCount.WithLabelValues(m.dropReason).Inc()
		}
	}()
	return out
}

func (m *matcherStage) processLogQL(e Entry) (Entry, bool) {
	for _, filter := range m.matchers {
		if !filter.Matches(string(e.Labels[model.LabelName(filter.Name)])) {
			return e, false
		}
	}

	if m.filter == nil || m.filter([]byte(e.Line)) {
		return e, true
	}
	return e, false
}

// Name implements Stage
func (m *matcherStage) Name() string {
	return StageTypeMatch
}
