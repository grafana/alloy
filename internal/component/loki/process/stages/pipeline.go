package stages

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
)

// StageConfig defines a single stage in a processing pipeline.
// We define these as pointers types so we can use reflection to check that
// exactly one is set.
type StageConfig struct {
	CRIConfig                    *CRIConfig                    `alloy:"cri,block,optional"`
	DecolorizeConfig             *DecolorizeConfig             `alloy:"decolorize,block,optional"`
	DockerConfig                 *DockerConfig                 `alloy:"docker,block,optional"`
	DropConfig                   *DropConfig                   `alloy:"drop,block,optional"`
	EventLogMessageConfig        *EventLogMessageConfig        `alloy:"eventlogmessage,block,optional"`
	GeoIPConfig                  *GeoIPConfig                  `alloy:"geoip,block,optional"`
	JSONConfig                   *JSONConfig                   `alloy:"json,block,optional"`
	LabelKeepConfig              *LabelKeepConfig              `alloy:"label_keep,block,optional"`
	LabelDropConfig              *LabelDropConfig              `alloy:"label_drop,block,optional"`
	LabelsConfig                 *LabelsConfig                 `alloy:"labels,block,optional"`
	LimitConfig                  *LimitConfig                  `alloy:"limit,block,optional"`
	LogfmtConfig                 *LogfmtConfig                 `alloy:"logfmt,block,optional"`
	LuhnFilterConfig             *LuhnFilterConfig             `alloy:"luhn,block,optional"`
	MatchConfig                  *MatchConfig                  `alloy:"match,block,optional"`
	MetricsConfig                *MetricsConfig                `alloy:"metrics,block,optional"`
	MultilineConfig              *MultilineConfig              `alloy:"multiline,block,optional"`
	OutputConfig                 *OutputConfig                 `alloy:"output,block,optional"`
	PackConfig                   *PackConfig                   `alloy:"pack,block,optional"`
	PatternConfig                *PatternConfig                `alloy:"pattern,block,optional"`
	RegexConfig                  *RegexConfig                  `alloy:"regex,block,optional"`
	ReplaceConfig                *ReplaceConfig                `alloy:"replace,block,optional"`
	SplitJSONConfig              *SplitJSONConfig              `alloy:"split_json,block,optional"`
	StaticLabelsConfig           *StaticLabelsConfig           `alloy:"static_labels,block,optional"`
	StructuredMetadata           *StructuredMetadataConfig     `alloy:"structured_metadata,block,optional"`
	StructuredMetadataDropConfig *StructuredMetadataDropConfig `alloy:"structured_metadata_drop,block,optional"`
	SamplingConfig               *SamplingConfig               `alloy:"sampling,block,optional"`
	TemplateConfig               *TemplateConfig               `alloy:"template,block,optional"`
	TenantConfig                 *TenantConfig                 `alloy:"tenant,block,optional"`
	TruncateConfig               *TruncateConfig               `alloy:"truncate,block,optional"`
	TimestampConfig              *TimestampConfig              `alloy:"timestamp,block,optional"`
	WindowsEventConfig           *WindowsEventConfig           `alloy:"windowsevent,block,optional"`
}

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

// Pipeline passes log entries down to each stage for mutation and/or label extraction.
// A Pipeline runs once: Start and Stop must each be called at most once, and a
// stopped Pipeline cannot be restarted.
type Pipeline struct {
	out   chan<- loki.Entry
	inner *pipeline

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPipeline creates a new log entry pipeline from a configuration
func NewPipeline(slogger *slog.Logger, stages []StageConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (*Pipeline, error) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pipeline{
		ctx:    ctx,
		cancel: cancel,
	}

	inner, err := newPipeline(slogger, registerer, minStability, stages, p.collect)
	if err != nil {
		return nil, err
	}

	p.inner = inner
	return p, nil
}

// collect ignores ctx on purpose. An entry that reaches collect has already
// been through every stage, so abandoning it when Stop cancels would lose data
// the pipeline accepted.
func (p *Pipeline) collect(_ context.Context, entries []Entry) error {
	for _, e := range entries {
		p.out <- e.Entry
	}
	return nil
}

// Start reads entries from in, passes them through the pipeline and forwards
// the results to out. Callers must keep consuming out until Stop returns.
func (p *Pipeline) Start(in chan loki.Entry, out chan<- loki.Entry) {
	p.out = out
	entries := make([]Entry, 0, 1)

	p.wg.Go(func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case e := <-in:
				entries = entries[:0]
				entries = append(entries, Entry{
					// NOTE: When entries pass through the pipeline
					// we always add all labels as extracted data.
					Extracted: make(map[string]any, len(e.Labels)),
					Entry:     e,
				})
				_ = p.inner.process(p.ctx, entries)
			}
		}
	})
}

// Stop stops reading from in and flushes any state the stages still hold, such
// as cri partial lines that never received their full line. Entries already
// accepted are forwarded to out before Stop returns.
func (p *Pipeline) Stop() {
	p.cancel()
	p.wg.Wait()
	p.inner.stop()
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
	for _, stream := range batch.Streams() {
		entries = slices.Grow(entries[:0], len(stream.Entries))

		for _, e := range stream.Entries {
			// Stages modify labels in place and the batch is only borrowed, so every
			// entry needs its own labels.
			// FIXME(kalleep): this clone will be removed when https://github.com/grafana/alloy/issues/6835 is implemented.
			entry := loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), stream.Created(), e)
			entries = append(entries, Entry{Extracted: make(map[string]any), Entry: entry})
		}

		if err := p.inner.process(ctx, entries); err != nil {
			return err
		}
	}
	return nil
}

// Stop flushes any state the stages still hold, such as cri partial lines that
// never received their full line. Callers must make sure no call to Consume is
// in flight or can start after this point. A racing Consume can buffer an entry
// after the flush has passed it, and nothing will forward it.
func (p *PipelineConsumer) Stop() {
	// TODO(kalleep): Release entries blocked inside a stage (e.g. stage.limit)
	// on shutdown, like Pipeline.Stop does.
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

		p.stages = append(p.stages, s)
		next = s.process
	}

	// We start stages after we have successfully built them all.
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
	// seed it again with any new labels.
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
