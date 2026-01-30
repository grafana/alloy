package stages

import (
	"fmt"
	"sync"

	"github.com/go-kit/log"
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
	logger    log.Logger
	stages    []Stage
	jobName   *string
	dropCount *prometheus.CounterVec
}

// NewPipeline creates a new log entry pipeline from a configuration
func NewPipeline(logger log.Logger, stages []StageConfig, jobName *string, registerer prometheus.Registerer, minStability featuregate.Stability) (*Pipeline, error) {
	st := []Stage{}
	for _, stage := range stages {
		newStage, err := New(logger, jobName, stage, registerer, minStability)
		if err != nil {
			return nil, fmt.Errorf("invalid stage config %w", err)
		}
		st = append(st, newStage)
	}
	return &Pipeline{
		logger:    log.With(logger, "component", "pipeline"),
		stages:    st,
		jobName:   jobName,
		dropCount: getDropCountMetric(registerer),
	}, nil
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

// Run implements Stage
func (p *Pipeline) Run(in chan Entry) chan Entry {
	in = RunWith(in, func(e Entry) Entry {
		// Initialize the extracted map with the initial labels (ie. "filename"),
		// so that stages can operate on initial labels too
		for labelName, labelValue := range e.Labels {
			e.Extracted[string(labelName)] = string(labelValue)
		}
		return e
	})
	// chain all stages together.
	for _, m := range p.stages {
		in = m.Run(in)
	}
	return in
}

// Name implements Stage
func (p *Pipeline) Name() string {
	return StageTypePipeline
}

// Cleanup implements Stage.
func (p *Pipeline) Cleanup() {
	for _, s := range p.stages {
		s.Cleanup()
	}
}

// Wrap implements EntryMiddleware
func (p *Pipeline) Wrap(next loki.EntryHandler) loki.EntryHandler {
	handlerIn := make(chan loki.Entry)
	nextChan := next.Chan()
	wg, once := sync.WaitGroup{}, sync.Once{}
	pipelineIn := make(chan Entry)
	pipelineOut := p.Run(pipelineIn)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for e := range pipelineOut {
			nextChan <- e.Entry
		}
	}()
	go func() {
		defer wg.Done()
		defer close(pipelineIn)
		for e := range handlerIn {
			pipelineIn <- Entry{
				Extracted: map[string]any{},
				Entry:     e,
			}
		}
	}()
	return loki.NewEntryHandler(handlerIn, func() {
		once.Do(func() { close(handlerIn) })
		wg.Wait()
		p.Cleanup()
	})
}

// Size gets the current number of stages in the pipeline
func (p *Pipeline) Size() int {
	return len(p.stages)
}
