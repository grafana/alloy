package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/client_golang/prometheus"
)

// NextFn forwards a batch of entries to whatever comes next in a pipeline.
type NextFn func(ctx context.Context, entries []Entry) error

// entryProcessor is a single step in a pipeline.
type entryProcessor interface {
	// process performs work on entries. The result is not returned.
	// Typically the implementations call a `NextFn` configured at construction time.
	// Errors are propagated.
	process(ctx context.Context, entries []Entry) error
}

type stopper interface {
	stop()
}

var _ entryProcessor = (*Pipeline2)(nil)

// Pipeline2 runs a batch of entries through a configured chain of stages,
// passing each batch from one stage to the next via direct function calls.
type Pipeline2 struct {
	next   NextFn
	stages []entryProcessor
}

func NewPipeline2(
	slogger *slog.Logger,
	registerer prometheus.Registerer,
	minStability featuregate.Stability,
	cfgs []StageConfig,
	next NextFn,
) (*Pipeline2, error) {

	var stages []entryProcessor

	// We build stages from the back so we can pass the correct next function
	// to the constructor.
	for _, cfg := range slices.Backward(cfgs) {
		s, err := newStageWithNextFn(slogger, cfg, registerer, minStability, next)
		if err != nil {
			return nil, fmt.Errorf("invalid stage config %w", err)
		}

		newStage, ok := s.(entryProcessor)
		if !ok {
			return nil, errors.New("stage has not been migrated to new interface")
		}

		stages = append(stages, newStage)
		next = newStage.process
	}

	return &Pipeline2{
		next:   next,
		stages: stages,
	}, nil
}

// ProcessBatch runs every entry in batch through the pipeline.
func (p *Pipeline2) ProcessBatch(ctx context.Context, batch loki.Batch) error {
	entries := make([]Entry, 0, batch.EntryLen())
	_ = batch.ConsumeStreams(func(stream loki.Stream, created int64) error {
		extracted := make(map[string]any, len(stream.Labels))
		for k, v := range stream.Labels {
			extracted[string(k)] = string(v)
		}

		for i, e := range stream.Entries {
			if i == len(stream.Entries)-1 {
				entries = append(entries, Entry{
					Extracted: extracted,
					Entry:     loki.NewEntryWithCreatedUnixMicro(stream.Labels, created, e),
				})
			} else {
				entries = append(entries, Entry{
					Extracted: maps.Clone(extracted),
					Entry:     loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), created, e),
				})
			}
		}
		return nil
	})

	return p.process(ctx, entries)
}

// ProcessEntry runs a single entry through the pipeline.
func (p *Pipeline2) ProcessEntry(ctx context.Context, entry loki.Entry) error {
	extracted := make(map[string]any, len(entry.Labels))
	for k, v := range entry.Labels {
		extracted[string(k)] = string(v)
	}

	return p.process(ctx, []Entry{
		{
			Extracted: extracted,
			Entry:     entry,
		},
	})
}

func (p *Pipeline2) process(ctx context.Context, entries []Entry) error {
	return p.next(ctx, entries)
}

func (p *Pipeline2) Stop() {
	// stages is stored in the reverse of its config order, so iterate
	// backwards to stop in the original, upstream-first order.
	for _, s := range slices.Backward(p.stages) {
		c, ok := s.(stopper)
		if ok {
			c.stop()
		}
	}
}
