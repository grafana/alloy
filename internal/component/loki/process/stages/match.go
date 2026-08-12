package stages

import (
	"errors"
	"fmt"
	"log/slog"

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

// MatchConfig contains the configuration for a match stage.
type MatchConfig struct {
	Selector     string        `alloy:"selector,attr"`
	Stages       []StageConfig `alloy:"stage,enum,optional"`
	Action       string        `alloy:"action,attr,optional"`
	PipelineName string        `alloy:"pipeline_name,attr,optional"`
	DropReason   string        `alloy:"drop_counter_reason,attr,optional"`
}

// validateMatcherConfig validates a match stage's MatchConfig.
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

// newMatcherStage creates a new match stage from config. Which of the two
// concrete types it returns is decided once, here, rather than at every
// call: a drop match, or a keep match whose nested pipeline is entirely
// made of SyncStages, becomes a *syncMatchStage (itself a SyncStage — no
// channel, ever). A keep match whose nested pipeline needs a channel (e.g.
// it contains a multiline stage) becomes a *asyncMatchStage (a ChannelStage,
// using the same channel that pipeline would have needed anyway).
func newMatcherStage(slogger *slog.Logger, config MatchConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (Stage, error) {
	selector, err := validateMatcherConfig(&config)
	if err != nil {
		return nil, err
	}

	filter, err := selector.Filter()
	if err != nil {
		return nil, fmt.Errorf("%v: %w", "error parsing pipeline", err)
	}

	if config.Action == MatchActionDrop {
		dropReason := "match_stage"
		if config.DropReason != "" {
			dropReason = config.DropReason
		}
		dropCount, err := getDropCountMetric(registerer)
		if err != nil {
			return nil, err
		}
		return &syncMatchStage{
			matchers:   selector.Matchers(),
			filter:     filter,
			action:     config.Action,
			dropReason: dropReason,
			dropCount:  dropCount,
		}, nil
	}

	pl, err := NewPipeline(slogger, config.Stages, registerer, minStability)
	if err != nil {
		return nil, fmt.Errorf("match stage failed to create pipeline from config %+v: %w", config, err)
	}

	if keepFn, ok := pl.SyncFunc(); ok {
		return &syncMatchStage{
			matchers: selector.Matchers(),
			filter:   filter,
			action:   config.Action,
			pipeline: pl,
			keepFn:   keepFn,
		}, nil
	}

	return &asyncMatchStage{
		matchers: selector.Matchers(),
		filter:   filter,
		pipeline: pl,
	}, nil
}

func getDropCountMetric(registerer prometheus.Registerer) (*prometheus.CounterVec, error) {
	dropCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_process_dropped_lines_total",
		Help: "A count of all log lines dropped as a result of a pipeline stage",
	}, []string{"reason"})
	err := registerer.Register(dropCount)
	if err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			dropCount = existing.ExistingCollector.(*prometheus.CounterVec)
		} else {
			return nil, err
		}
	}
	return dropCount, nil
}

// matchLogQL reports whether an entry matches a match stage's label
// matchers and line filter, shared by both syncMatchStage and
// asyncMatchStage.
func matchLogQL(matchers []*labels.Matcher, filter logql.Filter, e Entry) (Entry, bool) {
	for _, m := range matchers {
		if !m.Matches(string(e.Labels[model.LabelName(m.Name)])) {
			return e, false
		}
	}
	if filter == nil || filter([]byte(e.Line)) {
		return e, true
	}
	return e, false
}

// syncMatchStage is a match stage that never needs a channel of its own:
// either it's a drop match (which never has a nested pipeline), or it's a
// keep match whose nested pipeline is entirely made of SyncStages and so
// collapses into keepFn, a single function call.
type syncMatchStage struct {
	matchers []*labels.Matcher
	filter   logql.Filter
	action   string

	// dropReason/dropCount are only set when action == MatchActionDrop.
	dropReason string
	dropCount  *prometheus.CounterVec

	// pipeline/keepFn are only set when action == MatchActionKeep. pipeline
	// is kept so Cleanup/Stop can still reach whatever's nested inside the
	// match block; keepFn is pipeline's own fused Process call.
	pipeline *Pipeline
	keepFn   func(Entry) (Entry, bool)
}

func (m *syncMatchStage) Process(e Entry) (Entry, bool) {
	e, matched := matchLogQL(m.matchers, m.filter, e)
	if !matched {
		return e, false
	}
	if m.action == MatchActionDrop {
		m.dropCount.WithLabelValues(m.dropReason).Inc()
		return e, true
	}
	return m.keepFn(e)
}

func (m *syncMatchStage) Cleanup() {
	if m.pipeline != nil {
		m.pipeline.Cleanup()
	}
}

func (m *syncMatchStage) Stop() {
	if m.pipeline != nil {
		m.pipeline.Stop()
	}
}

// asyncMatchStage is a keep match whose nested pipeline needs a channel of
// its own (e.g. it contains a multiline stage), so the match needs one too.
type asyncMatchStage struct {
	matchers []*labels.Matcher
	filter   logql.Filter
	pipeline *Pipeline
}

func (m *asyncMatchStage) Run(in chan Entry) chan Entry {
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
			e, ok := matchLogQL(m.matchers, m.filter, e)
			if !ok {
				out <- e
				continue
			}
			next <- e
		}
	}()
	return out
}

func (m *asyncMatchStage) Cleanup() {
	m.pipeline.Cleanup()
}

func (m *asyncMatchStage) Stop() {
	m.pipeline.Stop()
}
