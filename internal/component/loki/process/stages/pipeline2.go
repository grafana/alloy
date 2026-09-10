package stages

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/client_golang/prometheus"
)

// nextFn forwards a batch of entries to whatever comes next in a pipeline.
type nextFn func(ctx context.Context, entries []Entry) error

// entryProcessor is a single step in a pipeline.
type entryProcessor interface {
	// process performs work on entries. The result is not returned.
	// Typically the implementations call a `NextFn` configured at construction time.
	// Errors are propagated.
	process(ctx context.Context, entries []Entry) error
}

// starter is implemented by stages that need to start background work (e.g.
// a goroutine) once the pipeline is fully built and guaranteed to run.
type starter interface {
	start()
}

// stopper is implemented by stages that need to flush buffered entries or
// release resources when the pipeline shuts down.
type stopper interface {
	stop()
}

var _ loki.Consumer = (*PipelineConsumer)(nil)

func NewPipelineConsumer(
	slogger *slog.Logger,
	registerer prometheus.Registerer,
	minStability featuregate.Stability,
	cfgs []StageConfig,
	consumer loki.Consumer,
) (*PipelineConsumer, error) {

	var (
		err error
		pc  = &PipelineConsumer{consumer: consumer}
	)

	pc.inner, err = newPipeline(slogger, registerer, minStability, cfgs, pc.collect)
	if err != nil {
		return nil, err
	}

	return pc, nil
}

type PipelineConsumer struct {
	inner    *pipeline
	consumer loki.Consumer
}

// Consume implements loki.Consumer.
func (p *PipelineConsumer) Consume(ctx context.Context, batch loki.Batch) error {
	entries := make([]Entry, 0, batch.EntryLen())
	return batch.ConsumeStreams(func(stream loki.Stream) error {
		entries = slices.Grow(entries[:0], len(stream.Entries))

		for i, e := range stream.Entries {
			if i == len(stream.Entries)-1 {
				entries = append(entries, Entry{
					Extracted: make(map[string]any, len(stream.Labels)),
					Entry:     loki.NewEntryWithCreatedUnixMicro(stream.Labels, stream.Created(), e),
				})
			} else {
				entries = append(entries, Entry{
					Extracted: make(map[string]any, len(stream.Labels)),
					//FIXME(kalleep): this clone will be removed when https://github.com/grafana/alloy/issues/6835 is implemented.
					Entry: loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), stream.Created(), e),
				})
			}
		}

		return p.inner.process(ctx, entries)
	})
}

func (p *PipelineConsumer) Stop() {
	p.inner.stop()
}

func (p *PipelineConsumer) collect(ctx context.Context, entries []Entry) error {
	batch := loki.NewBatch()
	for _, e := range entries {
		batch.AddEntry(e.Labels, e.Created(), e.Entry.Entry)
	}
	return p.consumer.Consume(ctx, batch)
}

var _ entryProcessor = (*pipeline)(nil)

// pipeline runs a batch of entries through a configured chain of stages,
// passing each batch from one stage to the next via direct function calls.
type pipeline struct {
	next   nextFn
	stages []entryProcessor
}

func newPipeline(
	slogger *slog.Logger,
	registerer prometheus.Registerer,
	minStability featuregate.Stability,
	cfgs []StageConfig,
	next nextFn,
) (*pipeline, error) {
	p := &pipeline{}

	// We build stages from the back so we can pass the correct next function
	// to the constructor.
	for _, cfg := range slices.Backward(cfgs) {
		s, err := newStageWithOpts(cfg, stageOpts{
			slogger:      slogger,
			registerer:   registerer,
			minStability: minStability,
			next:         next,
		})
		if err != nil {
			p.stop()
			return nil, fmt.Errorf("invalid stage config %w", err)
		}

		ep := s.(entryProcessor)
		p.stages = append(p.stages, ep)
		next = ep.process
	}

	// We start stages after we have sucessfully built them all.
	for _, s := range slices.Backward(p.stages) {
		if ss, ok := s.(starter); ok {
			ss.start()
		}
	}

	p.next = next
	return p, nil
}

func (p *pipeline) process(ctx context.Context, entries []Entry) error {
	// Seed extracted with labels. It is important to do it
	// here since a nested pipeline within a match stage needs to
	// seed it again whith any new labels.
	for i := range entries {
		for k, v := range entries[i].Labels {
			entries[i].Extracted[string(k)] = string(v)
		}
	}
	return p.next(ctx, entries)
}

func (p *pipeline) stop() {
	// stages is stored in the reverse of its config order, so iterate
	// backwards to stop in the original, upstream-first order.
	for _, s := range slices.Backward(p.stages) {
		c, ok := s.(stopper)
		if ok {
			c.stop()
		}
	}
}
