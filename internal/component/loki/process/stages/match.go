package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

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
func newMatchStage(config MatchConfig, opts stageOpts) (entryProcessor, error) {
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

		dropCount, err := getDropCountMetric(opts.registerer)
		if err != nil {
			return nil, err
		}

		return newMatchDropStage(filter, matchers, dropCount.WithLabelValues(dropReason), opts.next), nil
	default:
		return newMatchKeepStage(opts.slogger, opts.registerer, opts.minStability, config.Stages, filter, matchers, opts.next)
	}
}

var (
	_ entryProcessor = (*matchDropStage)(nil)
)

func newMatchDropStage(filter logql.Filter, matchers []*labels.Matcher, dropCount prometheus.Counter, next nextFn) *matchDropStage {
	return &matchDropStage{
		next:      next,
		filter:    filter,
		matchers:  matchers,
		dropCount: dropCount,
	}
}

type matchDropStage struct {
	next     nextFn
	filter   logql.Filter
	matchers []*labels.Matcher

	dropCount prometheus.Counter
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

var (
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
	next nextFn,
) (*matchKeepStage, error) {

	s := &matchKeepStage{
		next:     next,
		filter:   filter,
		matchers: matchers,
	}

	p, err := newPipeline(logger, reg, minStability, stages, s.collect)
	if err != nil {
		return nil, fmt.Errorf("match stage failed to create pipeline from config %+v: %w", stages, err)
	}

	s.pipeline = p

	return s, nil
}

type matchKeepStage struct {
	next nextFn

	pipeline *pipeline

	filter   logql.Filter
	matchers []*labels.Matcher
}

type matchMergeKey struct{}

// withMatchMerge stores a pointer to an entry slice that the inner pipeline
// output is appended to.
func withMatchMerge(ctx context.Context, ptr *[]Entry) context.Context {
	return context.WithValue(ctx, matchMergeKey{}, ptr)
}

// fromMatchMerge retrieves pointer to a entry slice set by withMatchMerge.
func fromMatchMerge(ctx context.Context) (*[]Entry, bool) {
	v := ctx.Value(matchMergeKey{})
	ptr, ok := v.(*[]Entry)
	return ptr, ok
}

// process implements stage.
func (m *matchKeepStage) process(ctx context.Context, entries []Entry) error {
	var (
		merged []Entry
		start  = -1
		end    int
	)

	for i, e := range entries {
		match := matchLogQL(e, m.matchers, m.filter)
		if match {
			// There is at least one match so entries are collected into merged.
			if merged == nil {
				merged = make([]Entry, 0, len(entries))
				merged = append(merged, entries[:i]...)
			}
			if start < 0 {
				start = i
			}
			end = i + 1
		}

		// Consecutive matched entries are processed together and in input order so
		// entries sharing a timestamp keep their original order.
		if start >= 0 && (!match || end == len(entries)) {
			// Set len and cap to end so that inner stages cannot overwrite the entries that follow.
			if err := m.pipeline.process(withMatchMerge(ctx, &merged), entries[start:end:end]); err != nil {
				return err
			}
			start = -1
		}

		if !match && merged != nil {
			merged = append(merged, e)
		}
	}

	if merged != nil {
		// Entries that took different branches can still share a stream so we need to
		// make sure they are sorted to prevent OOO.
		slices.SortStableFunc(merged, func(x, y Entry) int {
			return x.Timestamp.Compare(y.Timestamp)
		})
		entries = merged
	}

	if len(entries) == 0 {
		return nil
	}

	return m.next(ctx, entries)
}

// collected is passed as the next function to the inner pipeline.
func (m *matchKeepStage) collect(ctx context.Context, entries []Entry) error {
	// If context contains a pointer to a entry slice that means we called it directly.
	if buf, ok := fromMatchMerge(ctx); ok {
		*buf = append(*buf, entries...)
		return nil
	}

	return m.next(ctx, entries)
}

// stop implements stopper.
func (m *matchKeepStage) stop() {
	m.pipeline.stop()
}

func matchLogQL(e Entry, matchers []*labels.Matcher, filter logql.Filter) bool {
	for _, m := range matchers {
		if !m.Matches(string(e.Labels[model.LabelName(m.Name)])) {
			return false
		}
	}

	// FIXME(kalleep): With a line filter this converts every line to bytes, one
	// allocation per entry. We should change logql.Filter to take a string instead.
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
