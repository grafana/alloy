package stages

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
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

var _ syntax.Validator = (*StageConfig)(nil)

// Validate implements syntax.Validator.
func (s *StageConfig) Validate() error {
	var nonNilConfigs []string
	if s.CRIConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "cri")
	}
	if s.DecolorizeConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "decolorize")
	}
	if s.DockerConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "docker")
	}
	if s.DropConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "drop")
	}
	if s.EventLogMessageConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "eventlogmessage")
	}
	if s.GeoIPConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "geoip")
	}
	if s.JSONConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "json")
	}
	if s.LabelAllowConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "label_keep")
	}
	if s.LabelDropConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "label_drop")
	}
	if s.LabelsConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "labels")
	}
	if s.LimitConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "limit")
	}
	if s.LogfmtConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "logfmt")
	}
	if s.LuhnFilterConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "luhn")
	}
	if s.MatchConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "match")
	}
	if s.MetricsConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "metrics")
	}
	if s.MultilineConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "multiline")
	}
	if s.OutputConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "output")
	}
	if s.PackConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "pack")
	}
	if s.PatternConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "pattern")
	}
	if s.RegexConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "regex")
	}
	if s.ReplaceConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "replace")
	}
	if s.StaticLabelsConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "static_labels")
	}
	if s.StructuredMetadata != nil {
		nonNilConfigs = append(nonNilConfigs, "structured_metadata")
	}
	if s.StructuredMetadataDropConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "structured_metadata_drop")
	}
	if s.SamplingConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "sampling")
	}
	if s.TemplateConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "template")
	}
	if s.TenantConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "tenant")
	}
	if s.TruncateConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "truncate")
	}
	if s.TimestampConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "timestamp")
	}
	if s.WindowsEventConfig != nil {
		nonNilConfigs = append(nonNilConfigs, "windowsevent")
	}

	if len(nonNilConfigs) == 0 {
		return fmt.Errorf("empty stage config")
	}
	if len(nonNilConfigs) > 1 {
		return fmt.Errorf("stage config can only define one of %v", nonNilConfigs)
	}

	if s.CRIConfig != nil {
		return s.CRIConfig.Validate()
	}
	if s.DropConfig != nil {
		return s.DropConfig.Validate()
	}
	if s.EventLogMessageConfig != nil {
		return s.EventLogMessageConfig.Validate()
	}
	if s.GeoIPConfig != nil {
		return s.GeoIPConfig.Validate()
	}
	if s.JSONConfig != nil {
		return s.JSONConfig.Validate()
	}
	if s.LabelAllowConfig != nil {
		return s.LabelAllowConfig.Validate()
	}
	if s.LabelDropConfig != nil {
		return s.LabelDropConfig.Validate()
	}
	if s.LabelsConfig != nil {
		return s.LabelsConfig.Validate()
	}
	if s.LimitConfig != nil {
		return s.LimitConfig.Validate()
	}
	if s.LogfmtConfig != nil {
		return s.LogfmtConfig.Validate()
	}
	if s.LuhnFilterConfig != nil {
		return s.LuhnFilterConfig.Validate()
	}
	if s.MatchConfig != nil {
		return s.MatchConfig.Validate()
	}
	if s.MetricsConfig != nil {
		return s.MetricsConfig.Validate()
	}
	if s.MultilineConfig != nil {
		return s.MultilineConfig.Validate()
	}
	if s.OutputConfig != nil {
		return s.OutputConfig.Validate()
	}
	if s.PackConfig != nil {
		return s.PackConfig.Validate()
	}
	if s.PatternConfig != nil {
		return s.PatternConfig.Validate()
	}
	if s.RegexConfig != nil {
		return s.RegexConfig.Validate()
	}
	if s.ReplaceConfig != nil {
		return s.ReplaceConfig.Validate()
	}
	if s.StaticLabelsConfig != nil {
		return s.StaticLabelsConfig.Validate()
	}
	if s.StructuredMetadata != nil {
		return s.StructuredMetadata.Validate()
	}
	if s.StructuredMetadataDropConfig != nil {
		return s.StructuredMetadataDropConfig.Validate()
	}
	if s.SamplingConfig != nil {
		return s.SamplingConfig.Validate()
	}
	if s.TemplateConfig != nil {
		return s.TemplateConfig.Validate()
	}
	if s.TenantConfig != nil {
		return s.TenantConfig.Validate()
	}
	if s.TruncateConfig != nil {
		return s.TruncateConfig.Validate()
	}
	if s.TimestampConfig != nil {
		return s.TimestampConfig.Validate()
	}
	if s.WindowsEventConfig != nil {
		return s.WindowsEventConfig.Validate()
	}

	return nil
}

// Pipeline pass down a log entry to each stage for mutation and/or label extraction.
type Pipeline struct {
	stages    []Stage
	dropCount *prometheus.CounterVec
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
	return &Pipeline{
		stages:    st,
		dropCount: getDropCountMetric(registerer),
	}, nil
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
