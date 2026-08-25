package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/logql"
)

var (
	errSelectorRequired    = errors.New("selector statement required for match stage")
	errMatchRequiresStages = errors.New("match stage requires at least one additional stage to be defined in '- stages'")
	errSelectorSyntax      = errors.New("invalid selector syntax for match stage")
	errStagesWithDropLine  = errors.New("match stage configured to drop entries cannot contains stages")
	errUnknownMatchAction  = errors.New("match stage action should be 'keep' or 'drop'")
)

const (
	matchActionKeep = "keep"
	matchActionDrop = "drop"
)

// MatchConfig contains the configuration for a matcherStage
type MatchConfig struct {
	Selector string        `alloy:"selector,attr"`
	Stages   []StageConfig `alloy:"stage,enum,optional"`
	Action   string        `alloy:"action,attr,optional"`
	// PipelineName is unused but we need to keep it to not break configs.
	PipelineName string `alloy:"pipeline_name,attr,optional"`
	DropReason   string `alloy:"drop_counter_reason,attr,optional"`
}

// validateMatchConfig validates the MatcherConfig for the matcherStage
func validateMatchConfig(cfg MatchConfig) ([]*labels.Matcher, logql.Filter, error) {
	if cfg.Selector == "" {
		return nil, nil, errSelectorRequired
	}

	switch cfg.Action {
	case matchActionKeep, "":
		if len(cfg.Stages) == 0 {
			return nil, nil, errMatchRequiresStages
		}
	case matchActionDrop:
		if len(cfg.Stages) != 0 {
			return nil, nil, errStagesWithDropLine
		}
	default:
		return nil, nil, errUnknownMatchAction
	}

	selector, err := logql.ParseExpr(cfg.Selector)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errSelectorSyntax, err)
	}

	filter, err := selector.Filter()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errSelectorSyntax, err)
	}

	return selector.Matchers(), filter, nil
}

// newMatchStage creates a new matcherStage from config
func newMatchStage(slogger *slog.Logger, config MatchConfig, registerer prometheus.Registerer, minStability featuregate.Stability, next NextFn) (Stage, error) {
	matchers, filter, err := validateMatchConfig(config)
	if err != nil {
		return nil, err
	}

	switch config.Action {
	case matchActionDrop:
		dropReason := "match_stage"
		if config.DropReason != "" {
			dropReason = config.DropReason
		}

		dropCount, err := getDropCountMetric(registerer)
		if err != nil {
			return nil, err
		}

		return newMatchDropStage(filter, matchers, dropCount.WithLabelValues(dropReason), next), nil
	default:
		return newMatchKeepStage(slogger, registerer, minStability, config.Stages, filter, matchers, next)
	}
}

var (
	_ Stage          = (*matchDropStage)(nil)
	_ entryProcessor = (*matchDropStage)(nil)
)

func newMatchDropStage(filter logql.Filter, matchers []*labels.Matcher, dropCount prometheus.Counter, next NextFn) *matchDropStage {
	return &matchDropStage{
		next:      next,
		filter:    filter,
		matchers:  matchers,
		dropCount: dropCount,
	}
}

type matchDropStage struct {
	next     NextFn
	filter   logql.Filter
	matchers []*labels.Matcher

	dropCount prometheus.Counter
}

// Run implements Stage.
func (m *matchDropStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			if matchLogQL(e, m.matchers, m.filter) {
				m.dropCount.Inc()
				continue
			}
			out <- e
		}
	}()
	return out
}

// process implements stage.
func (m *matchDropStage) process(ctx context.Context, entries []Entry) error {
	var dst int
	for _, e := range entries {
		if matchLogQL(e, m.matchers, m.filter) {
			m.dropCount.Inc()
			continue
		}
		entries[dst] = e
		dst++
	}

	if dst == 0 {
		return nil
	}

	return m.next(ctx, entries[:dst])
}

// Cleanup implements Stage.
func (m *matchDropStage) Cleanup() {}

var (
	_ Stage   = (*matchKeepStage)(nil)
	_ Stopper = (*matchKeepStage)(nil)

	_ entryProcessor = (*matchKeepStage)(nil)
	_ stopper        = (*matchKeepStage)(nil)
)

func newMatchKeepStage(
	logger *slog.Logger,
	reg prometheus.Registerer,
	minStability featuregate.Stability,
	stages []StageConfig,
	filter logql.Filter,
	matchers []*labels.Matcher,
	next NextFn,
) (*matchKeepStage, error) {

	s := &matchKeepStage{
		next:     next,
		filter:   filter,
		matchers: matchers,
	}

	var (
		err error
		p1  *Pipeline
		p2  *Pipeline2
	)
	// NOTE: Next will only be set when stages are created from the new pipeline.
	// So if it's nil we create old pipeline and if it's set we create new pipeline.
	if next == nil {
		p1, err = NewPipeline(logger, stages, reg, minStability)
	} else {
		p2, err = NewPipeline2(logger, reg, minStability, stages, s.collect)
	}

	if err != nil {
		return nil, fmt.Errorf("match stage failed to create pipeline from config %+v: %w", stages, err)
	}

	s.pipeline = p1
	s.pipeline2 = p2

	return s, nil

}

type matchKeepStage struct {
	next NextFn

	pipeline  *Pipeline
	pipeline2 *Pipeline2

	filter   logql.Filter
	matchers []*labels.Matcher
}

// Run implements Stage.
func (m *matchKeepStage) Run(in chan Entry) chan Entry {
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
			if !matchLogQL(e, m.matchers, m.filter) {
				out <- e
				continue
			}
			next <- e
		}
	}()
	return out
}

// Stop implements Stopper.
func (m *matchKeepStage) Stop() {
	m.pipeline.Stop()
}

// Cleanup implements Stage.
func (m *matchKeepStage) Cleanup() {
	m.pipeline.Cleanup()
}

type pendingKey struct{}

// withPending scopes buffer to this call chain via ctx, so the nested
// pipeline's output can be routed back to the caller of process() instead
// of going straight to next.
func withPending(ctx context.Context, ptr *[]Entry) context.Context {
	return context.WithValue(ctx, pendingKey{}, ptr)
}

// fromPending retrieves the buffer set by withPending for this call chain.
func fromPending(ctx context.Context) (*[]Entry, bool) {
	v := ctx.Value(pendingKey{})
	ptr, ok := v.(*[]Entry)
	return ptr, ok
}

// process implements stage.
func (m *matchKeepStage) process(ctx context.Context, entries []Entry) error {
	var (
		dst     int
		matched []Entry
	)

	for _, e := range entries {
		if !matchLogQL(e, m.matchers, m.filter) {
			entries[dst] = e
			dst++
			continue
		}
		matched = append(matched, e)
	}

	if len(matched) > 0 {
		var buf []Entry
		if err := m.pipeline2.process(withPending(ctx, &buf), matched); err != nil {
			return err
		}
		dst += len(buf)
		entries = append(entries[:dst], buf...)
	}

	if dst == 0 {
		return nil
	}

	return m.next(ctx, entries[:dst])
}

// collected is passed as the next function to the inner pipeline.
func (m *matchKeepStage) collect(ctx context.Context, entries []Entry) error {
	// If context contains a pointer to a entry slice that means we called it directly.
	// In this case we should just set it to resulting entries from inner pipeline
	// so that process can merged it back.
	if buf, ok := fromPending(ctx); ok {
		*buf = entries
		return nil
	}

	return m.next(ctx, entries)
}

// stop implements stopper.
func (m *matchKeepStage) stop() {
	m.pipeline2.Stop()
}

func matchLogQL(e Entry, matchers []*labels.Matcher, filter logql.Filter) bool {
	for _, filter := range matchers {
		if !filter.Matches(string(e.Labels[model.LabelName(filter.Name)])) {
			return false
		}
	}

	if filter == nil || filter([]byte(e.Line)) {
		return true
	}
	return false
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
