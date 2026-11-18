package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/logql"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

const (
	matchActionKeep = "keep"
	matchActionDrop = "drop"
)

// MatchConfig contains the configuration for a matcherStage
type MatchConfig struct {
	Selector logql.Selector `alloy:"selector,attr"`
	Stages   []StageConfig  `alloy:"stage,enum,optional"`
	Action   string         `alloy:"action,attr,optional"`
	// PipelineName is unused but we need to keep it to not break configs.
	PipelineName string `alloy:"pipeline_name,attr,optional"`
	DropReason   string `alloy:"drop_counter_reason,attr,optional"`
}

var _ syntax.Validator = (*MatchConfig)(nil)

// Validate implements syntax.Validator.
func (cfg *MatchConfig) Validate() error {
	switch cfg.Action {
	case matchActionKeep, "":
		if len(cfg.Stages) == 0 {
			return errors.New(`stage.match requires at least one nested stage when action is "keep"`)
		}
	case matchActionDrop:
		if len(cfg.Stages) != 0 {
			return errors.New(`stage.match cannot have nested stages when action is "drop"`)
		}
	default:
		return fmt.Errorf(`stage.match action must be "keep" or "drop", got %q`, cfg.Action)
	}

	return nil
}

// newMatchStage creates a new matcherStage from config
func newMatchStage(config MatchConfig, opts stageOpts) (entryProcessor, error) {
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

		return newMatchDropStage(config.Selector, dropCount.WithLabelValues(dropReason), opts.next), nil
	default:
		return newMatchKeepStage(opts.slogger, opts.registerer, opts.minStability, config.Stages, config.Selector, opts.next)
	}
}

var (
	_ entryProcessor = (*matchDropStage)(nil)
)

func newMatchDropStage(selector logql.Selector, dropCount prometheus.Counter, next nextFn) *matchDropStage {
	return &matchDropStage{
		next:      next,
		selector:  selector,
		dropCount: dropCount,
	}
}

type matchDropStage struct {
	next     nextFn
	selector logql.Selector

	dropCount prometheus.Counter
}

// process implements stage.
func (m *matchDropStage) process(ctx context.Context, entries []Entry) error {
	var dst int
	for _, e := range entries {
		if m.selector.Matches(e.Labels, e.Line) {
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
	selector logql.Selector,
	next nextFn,
) (*matchKeepStage, error) {

	s := &matchKeepStage{
		next:     next,
		selector: selector,
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

	selector logql.Selector
}

type matchMergeKey struct{}

// matchMergeCtx carries the entry slice that the inner pipeline output is appended to.
type matchMergeCtx struct {
	context.Context
	entries []Entry
}

func (c *matchMergeCtx) Value(key any) any {
	if key == (matchMergeKey{}) {
		return c
	}
	return c.Context.Value(key)
}

// fromMatchMerge retrieves the matchMergeCtx closest to ctx, if any.
func fromMatchMerge(ctx context.Context) (*matchMergeCtx, bool) {
	mc, ok := ctx.Value(matchMergeKey{}).(*matchMergeCtx)
	return mc, ok
}

// process implements stage.
func (m *matchKeepStage) process(ctx context.Context, entries []Entry) error {
	var (
		start = -1
		end   int
		// Only allocated when there is at least one match.
		merged *matchMergeCtx
	)

	for i, e := range entries {
		match := m.selector.Matches(e.Labels, e.Line)
		if match {
			// There is at least one match so entries are collected into merged.
			if merged == nil {
				merged = &matchMergeCtx{Context: ctx, entries: make([]Entry, 0, len(entries))}
				merged.entries = append(merged.entries, entries[:i]...)
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
			if err := m.pipeline.process(merged, entries[start:end:end]); err != nil {
				return err
			}
			start = -1
		}

		if !match && merged != nil {
			merged.entries = append(merged.entries, e)
		}
	}

	if merged != nil {
		// Entries that took different branches can still share a stream so we need to
		// make sure they are sorted to prevent OOO.
		slices.SortStableFunc(merged.entries, func(x, y Entry) int {
			return x.Timestamp.Compare(y.Timestamp)
		})
		entries = merged.entries
	}

	if len(entries) == 0 {
		return nil
	}

	return m.next(ctx, entries)
}

// collected is passed as the next function to the inner pipeline.
func (m *matchKeepStage) collect(ctx context.Context, entries []Entry) error {
	// If context contains a matchMergeCtx that means we called it directly.
	if mc, ok := fromMatchMerge(ctx); ok {
		mc.entries = append(mc.entries, entries...)
		return nil
	}

	return m.next(ctx, entries)
}

// stop implements stopper.
func (m *matchKeepStage) stop() {
	m.pipeline.stop()
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
