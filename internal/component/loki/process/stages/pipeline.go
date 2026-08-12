package stages

import (
	"context"
	"fmt"
	"log/slog"
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
	LabelAllowConfig             *LabelAllowConfig             `alloy:"label_keep,block,optional"`
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

// Pipeline pass down a log entry to each stage for mutation and/or label extraction.
type Pipeline struct {
	stages    []Stage
	dropCount *prometheus.CounterVec

	// syncFn is non-nil only if every stage is a SyncStage, in which case
	// it's the whole pipeline fused into a single function: see
	// buildSyncFunc and SyncFunc.
	syncFn func(Entry) (Entry, bool)
}

// NewPipeline creates a new log entry pipeline from a configuration
func NewPipeline(slogger *slog.Logger, stages []StageConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (*Pipeline, error) {
	st := []Stage{}
	for _, stage := range stages {
		newStage, err := New(slogger, stage, registerer, minStability)
		if err != nil {
			return nil, fmt.Errorf("invalid stage config %w", err)
		}
		st = append(st, newStage)
	}
	dropCount, err := getDropCountMetric(registerer)
	if err != nil {
		return nil, err
	}

	return &Pipeline{
		stages:    st,
		dropCount: dropCount,
		syncFn:    buildSyncFunc(st),
	}, nil
}

// buildSyncFunc returns a single function fusing initEntry with every
// stage's Process, or nil if any stage isn't a SyncStage (i.e. needs a
// channel of its own).
func buildSyncFunc(stages []Stage) func(Entry) (Entry, bool) {
	fns := make([]func(Entry) (Entry, bool), 0, len(stages)+1)
	fns = append(fns, initEntry)
	for _, s := range stages {
		ss, ok := s.(SyncStage)
		if !ok {
			return nil
		}
		fns = append(fns, ss.Process)
	}
	return composeFuncs(fns)
}

// SyncFunc returns the pipeline fused into a single function, and whether
// that was possible (i.e. every stage in it is a SyncStage). newMatcherStage
// uses this to decide whether a "keep" match's nested pipeline can become a
// syncMatchStage (fused into its own Process call) or must be an
// asyncMatchStage (needing its own channel).
func (p *Pipeline) SyncFunc() (func(Entry) (Entry, bool), bool) {
	return p.syncFn, p.syncFn != nil
}

// Start will start the pipeline and forward entries to next.
// The returned EntryHandler should be used to pass entries through the pipeline.
func (p *Pipeline) Start(in chan loki.Entry, out chan<- loki.Entry) loki.EntryHandler {
	ctx, cancel := context.WithCancel(context.Background())

	pipelineIn := make(chan Entry)
	pipelineOut := p.Run(pipelineIn)

	var (
		wg   sync.WaitGroup
		once sync.Once
	)

	wg.Go(func() {
		for e := range pipelineOut {
			out <- e.Entry
		}
	})

	wg.Go((func() {
		defer close(pipelineIn)
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-in:
				pipelineIn <- Entry{
					// NOTE: When entires pass through the pipeline
					// we always add all labels as extracted data.
					Extracted: make(map[string]any, len(e.Labels)),
					Entry:     e,
				}
			}
		}
	}))

	return loki.NewEntryHandler(in, func() {
		once.Do(func() {
			cancel()
			p.Stop()
		})
		wg.Wait()
		p.Cleanup()
	})
}

// initEntry initializes the extracted map with the initial labels (ie.
// "filename"), so that stages can operate on initial labels too. It is the
// first step of every pipeline, fused in like any other SyncStage (see Run)
// instead of getting its own channel and goroutine.
func initEntry(e Entry) (Entry, bool) {
	for labelName, labelValue := range e.Labels {
		e.Extracted[string(labelName)] = string(labelValue)
	}
	return e, false
}

// Run turns the pipeline into a single input/output channel pair for
// Start (and tests) to drive.
//
// Naively, this would give every stage its own channel and goroutine,
// chained in sequence. For a pipeline built from many stages (e.g. one
// generated from many independent stage.match rules), that means every
// entry pays for a goroutine handoff per stage even though most stages are
// pure, synchronous, single-entry transforms with no need for a dedicated
// goroutine.
//
// If the whole pipeline is a SyncStage (syncFn != nil, the common case),
// this is just that one function wrapped in a single goroutine. Otherwise,
// it fuses maximal runs of SyncStages into a single goroutine and channel
// hop each (see composeFuncs), and only drops into a stage's own Run (its
// own channel and goroutine) for the ChannelStages that actually need one,
// such as multiline's wall-clock-based buffering.
func (p *Pipeline) Run(in chan Entry) chan Entry {
	if p.syncFn != nil {
		return RunWithSkip(in, p.syncFn)
	}

	var fused []func(Entry) (Entry, bool)
	flush := func() {
		if len(fused) == 0 {
			return
		}
		in = RunWithSkip(in, composeFuncs(fused))
		fused = nil
	}

	fused = append(fused, initEntry)
	for _, s := range p.stages {
		if ss, ok := s.(SyncStage); ok {
			fused = append(fused, ss.Process)
			continue
		}
		flush()
		in = s.(ChannelStage).Run(in)
	}
	flush()
	return in
}

// Cleanup implements Stage.
func (p *Pipeline) Cleanup() {
	for _, s := range p.stages {
		s.Cleanup()
	}
}

// Stop implements Stopper.
func (p *Pipeline) Stop() {
	for _, s := range p.stages {
		stopper, ok := s.(Stopper)
		if !ok {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			stopper.Stop()
		}()
	}
}

// RunWithSkipOrSendMany is RunWithSkip for a process function that can send
// zero, one, or many entries for a single input entry (e.g. split_json
// fanning one entry out into several).
func RunWithSkipOrSendMany(input chan Entry, process func(e Entry) ([]Entry, bool)) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range input {
			results, skip := process(e)
			if skip {
				continue
			}
			for _, result := range results {
				out <- result
			}
		}
	}()

	return out
}

// RunWithSkip reads entries from the input channel, mutates each with the
// process function, and forwards it to the output channel unless process
// returns skip=true, in which case the entry is dropped instead.
func RunWithSkip(input chan Entry, process func(Entry) (Entry, bool)) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range input {
			e, skip := process(e)
			if skip {
				continue
			}
			out <- e
		}
	}()
	return out
}

// composeFuncs chains single-entry process functions into one, in the same
// skip convention as RunWithSkip: skip=true short-circuits the rest of the
// chain. This only handles 1:1 stages; a stage that can fan one entry out
// into several (split_json, cri) is a ChannelStage, not a SyncStage, and so
// never reaches here, keeping this allocation-free.
func composeFuncs(fns []func(Entry) (Entry, bool)) func(Entry) (Entry, bool) {
	return func(e Entry) (Entry, bool) {
		for _, fn := range fns {
			var skip bool
			e, skip = fn(e)
			if skip {
				return e, true
			}
		}
		return e, false
	}
}
