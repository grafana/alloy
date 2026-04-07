package stages

import (
	"errors"
	"fmt"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/logql"
)

// Configuration errors.
var (
	ErrSelectorRequired    = errors.New("selector statement required for match stage")
	ErrMatchRequiresStages = errors.New("match stage requires at least one additional stage to be defined in '- stages'")
	ErrSelectorSyntax      = errors.New("invalid selector syntax for match stage")
	ErrStagesWithDropLine  = errors.New("match stage configured to drop entries cannot contains stages")
	ErrUnknownMatchAction  = errors.New("match stage action should be 'keep' or 'drop'")

	MatchActionKeep = "keep"
	MatchActionDrop = "drop"
)

// MatchConfig contains the configuration for a matcherStage
type MatchConfig struct {
	Selector     string        `alloy:"selector,attr"                     json:"selector"`
	Stages       []StageConfig `alloy:"stage,enum,optional"               json:"stages,omitempty"`
	Action       string        `alloy:"action,attr,optional"              json:"action,omitempty"`
	PipelineName string        `alloy:"pipeline_name,attr,optional"       json:"pipelineName,omitempty"`
	DropReason   string        `alloy:"drop_counter_reason,attr,optional" json:"dropReason,omitempty"`
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

	if cfg.Action == MatchActionKeep && len(cfg.Stages) == 0 {
		return nil, ErrMatchRequiresStages
	}
	if cfg.Action == MatchActionDrop && len(cfg.Stages) != 0 {
		return nil, ErrStagesWithDropLine
	}

	selector, err := logql.ParseExpr(cfg.Selector)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", ErrSelectorSyntax, err)
	}
	return selector, nil
}

// newMatcherStage creates a new matcherStage from config
func newMatcherStage(logger log.Logger, config MatchConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (Stage, error) {
	selector, err := validateMatcherConfig(&config)
	if err != nil {
		return nil, err
	}

	var pl *Pipeline
	if config.Action == MatchActionKeep {
		var err error
		pl, err = NewPipeline(logger, config.Stages, registerer, minStability)
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
		pipeline:   pl,
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
		// TODO: This code should neither panic nor use AlreadyRegisteredError.
		//       Register it without these, and return error if it fails.
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
	pipeline   *Pipeline
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
	outNext := m.pipeline.Run(next)
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

func (m *matcherStage) Cleanup() {
	if m.pipeline != nil {
		m.pipeline.Cleanup()
	}
}

// canProcessSync implements syncChecker. It reports whether this stage can
// participate in a synchronous pipeline: drop actions never need a nested
// pipeline, and keep actions require their nested pipeline to be sync-capable.
func (m *matcherStage) canProcessSync() bool {
	if m.action == MatchActionDrop {
		return true
	}
	return m.pipeline != nil && m.pipeline.CanProcessSync()
}

// ProcessEntry implements SyncStage.
func (m *matcherStage) ProcessEntry(e Entry) []Entry {
	switch m.action {
	case MatchActionDrop:
		if _, ok := m.processLogQL(e); ok {
			m.dropCount.WithLabelValues(m.dropReason).Inc()
			return nil
		}
		return []Entry{e}
	case MatchActionKeep:
		e, ok := m.processLogQL(e)
		if !ok {
			// Entry does not match the selector: pass through unchanged.
			return []Entry{e}
		}
		// Entry matches: run it through the nested pipeline.
		// CanProcessSync guarantees all nested stages implement SyncStage.
		return m.pipeline.processEntryFull(e)
	}
	panic("matcherStage: unexpected action " + m.action)
}
