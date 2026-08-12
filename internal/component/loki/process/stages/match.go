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

// MatchConfig contains the configuration for a matcherStage
type MatchConfig struct {
	Selector     string        `alloy:"selector,attr"`
	Stages       []StageConfig `alloy:"stage,enum,optional"`
	Action       string        `alloy:"action,attr,optional"`
	PipelineName string        `alloy:"pipeline_name,attr,optional"`
	DropReason   string        `alloy:"drop_counter_reason,attr,optional"`
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
func newMatcherStage(slogger *slog.Logger, config MatchConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (Stage, error) {
	selector, err := validateMatcherConfig(&config)
	if err != nil {
		return nil, err
	}

	var pl *Pipeline
	if config.Action == MatchActionKeep {
		var err error
		pl, err = NewPipeline(slogger, config.Stages, registerer, minStability)
		if err != nil {
			return nil, fmt.Errorf("match stage failed to create pipeline from config %+v: %w", config, err)
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

	dropCount, err := getDropCountMetric(registerer)
	if err != nil {
		return nil, err
	}

	m := &matcherStage{
		dropReason: dropReason,
		dropCount:  dropCount,
		matchers:   selector.Matchers(),
		pipeline:   pl,
		action:     config.Action,
		filter:     filter,
	}

	// Cache the nested pipeline's narrowly-fused function once, if it has
	// one: every stage nested under this "keep" match (including further
	// nested match blocks) needs to itself qualify for narrow fusion (see
	// trySyncNarrow) for the whole thing to collapse into a single function
	// call. If it doesn't, syncNarrowFn stays nil and this match stage
	// falls back to its existing channel-based runKeep.
	if pl != nil {
		m.syncNarrowFn, _ = pl.trySyncNarrow()
	}

	return m, nil
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

// matcherStage applies Label matchers to determine if the include stages should be run
type matcherStage struct {
	dropReason string
	dropCount  *prometheus.CounterVec
	matchers   []*labels.Matcher
	filter     logql.Filter
	pipeline   *Pipeline
	action     string

	// syncNarrowFn is pipeline's narrowly-fused function, cached once at
	// construction; nil if the nested pipeline doesn't qualify (see
	// trySyncNarrow) or for MatchActionDrop, which has no nested pipeline.
	syncNarrowFn func(Entry) (Entry, bool)
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

// trySyncNarrow implements syncNarrowCapable. A drop match always
// qualifies (it has no nested pipeline); a keep match qualifies only if
// its nested pipeline does, i.e. syncNarrowFn was successfully cached at
// construction.
func (m *matcherStage) trySyncNarrow() (func(Entry) (Entry, bool), bool) {
	switch m.action {
	case MatchActionDrop:
		return m.processDropSync, true
	case MatchActionKeep:
		if m.syncNarrowFn == nil {
			return nil, false
		}
		return m.processKeepSync, true
	}
	panic("unexpected action")
}

func (m *matcherStage) processDropSync(e Entry) (Entry, bool) {
	e, matched := m.processLogQL(e)
	if !matched {
		return e, false
	}
	m.dropCount.WithLabelValues(m.dropReason).Inc()
	return e, true
}

func (m *matcherStage) processKeepSync(e Entry) (Entry, bool) {
	e, matched := m.processLogQL(e)
	if !matched {
		return e, false
	}
	return m.syncNarrowFn(e)
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

func (m *matcherStage) Stop() {
	if m.pipeline != nil { // nil for MatchActionDrop matchers, see Cleanup
		m.pipeline.Stop()
	}
}
