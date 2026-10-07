package stages

import (
	"context"
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

// RunWith will read from the input channel entries, mutate them with the process function and returns them via the output channel.
func RunWith(input chan Entry, process func(e Entry) Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range input {
			out <- process(e)
		}
	}()
	return out
}

// RunWithSkipOrSendMany same as RunWith, except it handles sending multiple entries at the same time and it wil skip
// sending the batch to output channel, if `process` functions returns `skip` true.
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
