package stages

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
)

// StageConfig defines a single stage in a processing pipeline.
// We define these as pointers types so we can use reflection to check that
// exactly one is set.
type StageConfig struct {
	CRIConfig                    *CRIConfig                    `alloy:"cri,block,optional"                    json:"cri,omitempty"`
	DecolorizeConfig             *DecolorizeConfig             `alloy:"decolorize,block,optional"             json:"decolorize,omitempty"`
	DockerConfig                 *DockerConfig                 `alloy:"docker,block,optional"                 json:"docker,omitempty"`
	DropConfig                   *DropConfig                   `alloy:"drop,block,optional"                   json:"drop,omitempty"`
	EventLogMessageConfig        *EventLogMessageConfig        `alloy:"eventlogmessage,block,optional"        json:"eventlogmessage,omitempty"`
	GeoIPConfig                  *GeoIPConfig                  `alloy:"geoip,block,optional"                  json:"geoip,omitempty"`
	JSONConfig                   *JSONConfig                   `alloy:"json,block,optional"                   json:"json,omitempty"`
	LabelAllowConfig             *LabelAllowConfig             `alloy:"label_keep,block,optional"             json:"label_keep,omitempty"`
	LabelDropConfig              *LabelDropConfig              `alloy:"label_drop,block,optional"             json:"label_drop,omitempty"`
	LabelsConfig                 *LabelsConfig                 `alloy:"labels,block,optional"                 json:"labels,omitempty"`
	LimitConfig                  *LimitConfig                  `alloy:"limit,block,optional"                  json:"limit,omitempty"`
	LogfmtConfig                 *LogfmtConfig                 `alloy:"logfmt,block,optional"                 json:"logfmt,omitempty"`
	LuhnFilterConfig             *LuhnFilterConfig             `alloy:"luhn,block,optional"                   json:"luhn,omitempty"`
	MatchConfig                  *MatchConfig                  `alloy:"match,block,optional"                  json:"match,omitempty"`
	MetricsConfig                *MetricsConfig                `alloy:"metrics,block,optional"                json:"metrics,omitempty"`
	MultilineConfig              *MultilineConfig              `alloy:"multiline,block,optional"              json:"multiline,omitempty"`
	OutputConfig                 *OutputConfig                 `alloy:"output,block,optional"                 json:"output,omitempty"`
	PackConfig                   *PackConfig                   `alloy:"pack,block,optional"                   json:"pack,omitempty"`
	PatternConfig                *PatternConfig                `alloy:"pattern,block,optional"                json:"pattern,omitempty"`
	RegexConfig                  *RegexConfig                  `alloy:"regex,block,optional"                  json:"regex,omitempty"`
	ReplaceConfig                *ReplaceConfig                `alloy:"replace,block,optional"                json:"replace,omitempty"`
	StaticLabelsConfig           *StaticLabelsConfig           `alloy:"static_labels,block,optional"          json:"static_labels,omitempty"`
	StructuredMetadata           *StructuredMetadataConfig     `alloy:"structured_metadata,block,optional"    json:"structured_metadata,omitempty"`
	StructuredMetadataDropConfig *StructuredMetadataDropConfig `alloy:"structured_metadata_drop,block,optional" json:"structured_metadata_drop,omitempty"`
	SamplingConfig               *SamplingConfig               `alloy:"sampling,block,optional"               json:"sampling,omitempty"`
	TemplateConfig               *TemplateConfig               `alloy:"template,block,optional"               json:"template,omitempty"`
	TenantConfig                 *TenantConfig                 `alloy:"tenant,block,optional"                 json:"tenant,omitempty"`
	TruncateConfig               *TruncateConfig               `alloy:"truncate,block,optional"               json:"truncate,omitempty"`
	TimestampConfig              *TimestampConfig              `alloy:"timestamp,block,optional"              json:"timestamp,omitempty"`
	WindowsEventConfig           *WindowsEventConfig           `alloy:"windowsevent,block,optional"           json:"windowsevent,omitempty"`
}

// PodLogsDropConfig is the drop stage config for use in the PodLogs CRD.
// It mirrors DropConfig but replaces OlderThan (time.Duration, which marshals
// as a nanosecond int64 in JSON) with a human-readable duration string like
// "5m" or "1h30s".
type PodLogsDropConfig struct {
	DropReason string           `json:"dropReason,omitempty"`
	Source     string           `json:"source,omitempty"`
	Value      string           `json:"value,omitempty"`
	Separator  string           `json:"separator,omitempty"`
	Expression string           `json:"expression,omitempty"`
	OlderThan  string           `json:"olderThan,omitempty"` // human-readable duration e.g. "5m"
	LongerThan units.Base2Bytes `json:"longerThan,omitempty"`
}

// toDropConfig converts a PodLogsDropConfig to DropConfig.
// Returns an error if OlderThan is set but cannot be parsed as a duration.
func (c *PodLogsDropConfig) toDropConfig() (DropConfig, error) {
	cfg := DropConfig{
		DropReason: c.DropReason,
		Source:     c.Source,
		Value:      c.Value,
		Separator:  c.Separator,
		Expression: c.Expression,
		LongerThan: c.LongerThan,
	}
	if c.OlderThan != "" {
		d, err := time.ParseDuration(c.OlderThan)
		if err != nil {
			return DropConfig{}, fmt.Errorf("invalid olderThan duration %q: %w", c.OlderThan, err)
		}
		cfg.OlderThan = d
	}
	return cfg, nil
}

// PodLogsMatchConfig is the match stage config for use in the PodLogs CRD.
// It mirrors MatchConfig but uses PodLogsStageConfig for nested stages so that
// incompatible stage types (multiline, windowsevent, eventlogmessage) are
// excluded recursively.
type PodLogsMatchConfig struct {
	Selector     string               `json:"selector"`
	Stages       []PodLogsStageConfig `json:"stages,omitempty"`
	Action       string               `json:"action,omitempty"`
	PipelineName string               `json:"pipelineName,omitempty"`
	DropReason   string               `json:"dropReason,omitempty"`
}

// toMatchConfig converts a PodLogsMatchConfig to MatchConfig for use with newMatcherStage.
func (c *PodLogsMatchConfig) toMatchConfig() (MatchConfig, error) {
	converted, err := ConvertPodLogsStages(c.Stages)
	if err != nil {
		return MatchConfig{}, err
	}
	return MatchConfig{
		Selector:     c.Selector,
		Stages:       converted,
		Action:       c.Action,
		PipelineName: c.PipelineName,
		DropReason:   c.DropReason,
	}, nil
}

// PodLogsStageConfig defines a single processing stage for use in the PodLogs CRD.
// It mirrors StageConfig but excludes stages that are incompatible with a shared
// per-PodLogs pipeline:
//   - multiline: log lines from different pods interleave in the shared pipeline,
//     causing incorrect multi-line merging across pod boundaries.
//   - windowsevent / eventlogmessage: not applicable to Linux pod logs.
type PodLogsStageConfig struct {
	CRIConfig                    *CRIConfig                    `json:"cri,omitempty"`
	DecolorizeConfig             *DecolorizeConfig             `json:"decolorize,omitempty"`
	DockerConfig                 *DockerConfig                 `json:"docker,omitempty"`
	DropConfig                   *PodLogsDropConfig            `json:"drop,omitempty"`
	GeoIPConfig                  *GeoIPConfig                  `json:"geoip,omitempty"`
	JSONConfig                   *JSONConfig                   `json:"json,omitempty"`
	LabelAllowConfig             *LabelAllowConfig             `json:"label_keep,omitempty"`
	LabelDropConfig              *LabelDropConfig              `json:"label_drop,omitempty"`
	LabelsConfig                 *LabelsConfig                 `json:"labels,omitempty"`
	LimitConfig                  *LimitConfig                  `json:"limit,omitempty"`
	LogfmtConfig                 *LogfmtConfig                 `json:"logfmt,omitempty"`
	LuhnFilterConfig             *LuhnFilterConfig             `json:"luhn,omitempty"`
	MatchConfig                  *PodLogsMatchConfig           `json:"match,omitempty"`
	MetricsConfig                *MetricsConfig                `json:"metrics,omitempty"`
	OutputConfig                 *OutputConfig                 `json:"output,omitempty"`
	PackConfig                   *PackConfig                   `json:"pack,omitempty"`
	PatternConfig                *PatternConfig                `json:"pattern,omitempty"`
	RegexConfig                  *RegexConfig                  `json:"regex,omitempty"`
	ReplaceConfig                *ReplaceConfig                `json:"replace,omitempty"`
	SamplingConfig               *SamplingConfig               `json:"sampling,omitempty"`
	StaticLabelsConfig           *StaticLabelsConfig           `json:"static_labels,omitempty"`
	StructuredMetadata           *StructuredMetadataConfig     `json:"structured_metadata,omitempty"`
	StructuredMetadataDropConfig *StructuredMetadataDropConfig `json:"structured_metadata_drop,omitempty"`
	TemplateConfig               *TemplateConfig               `json:"template,omitempty"`
	TenantConfig                 *TenantConfig                 `json:"tenant,omitempty"`
	TimestampConfig              *TimestampConfig              `json:"timestamp,omitempty"`
	TruncateConfig               *TruncateConfig               `json:"truncate,omitempty"`
}

// ToStageConfig converts a PodLogsStageConfig to the full StageConfig for use
// with NewPipeline. Returns an error if any field value is invalid (e.g. an
// unparseable duration in a drop stage).
func (c PodLogsStageConfig) ToStageConfig() (StageConfig, error) {
	var dropConfig *DropConfig
	if c.DropConfig != nil {
		converted, err := c.DropConfig.toDropConfig()
		if err != nil {
			return StageConfig{}, err
		}
		dropConfig = &converted
	}
	var matchConfig *MatchConfig
	if c.MatchConfig != nil {
		converted, err := c.MatchConfig.toMatchConfig()
		if err != nil {
			return StageConfig{}, err
		}
		matchConfig = &converted
	}
	return StageConfig{
		CRIConfig:                    c.CRIConfig,
		DecolorizeConfig:             c.DecolorizeConfig,
		DockerConfig:                 c.DockerConfig,
		DropConfig:                   dropConfig,
		GeoIPConfig:                  c.GeoIPConfig,
		JSONConfig:                   c.JSONConfig,
		LabelAllowConfig:             c.LabelAllowConfig,
		LabelDropConfig:              c.LabelDropConfig,
		LabelsConfig:                 c.LabelsConfig,
		LimitConfig:                  c.LimitConfig,
		LogfmtConfig:                 c.LogfmtConfig,
		LuhnFilterConfig:             c.LuhnFilterConfig,
		MatchConfig:                  matchConfig,
		MetricsConfig:                c.MetricsConfig,
		OutputConfig:                 c.OutputConfig,
		PackConfig:                   c.PackConfig,
		PatternConfig:                c.PatternConfig,
		RegexConfig:                  c.RegexConfig,
		ReplaceConfig:                c.ReplaceConfig,
		SamplingConfig:               c.SamplingConfig,
		StaticLabelsConfig:           c.StaticLabelsConfig,
		StructuredMetadata:           c.StructuredMetadata,
		StructuredMetadataDropConfig: c.StructuredMetadataDropConfig,
		TemplateConfig:               c.TemplateConfig,
		TenantConfig:                 c.TenantConfig,
		TimestampConfig:              c.TimestampConfig,
		TruncateConfig:               c.TruncateConfig,
	}, nil
}

// ConvertPodLogsStages converts a slice of PodLogsStageConfig to []StageConfig
// for use with NewPipeline.
func ConvertPodLogsStages(in []PodLogsStageConfig) ([]StageConfig, error) {
	out := make([]StageConfig, len(in))
	for i, s := range in {
		converted, err := s.ToStageConfig()
		if err != nil {
			return nil, err
		}
		out[i] = converted
	}
	return out, nil
}

// Pipeline pass down a log entry to each stage for mutation and/or label extraction.
type Pipeline struct {
	logger    log.Logger
	stages    []Stage
	dropCount *prometheus.CounterVec
}

// NewPipeline creates a new log entry pipeline from a configuration
func NewPipeline(logger log.Logger, stages []StageConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (*Pipeline, error) {
	st := []Stage{}
	for _, stage := range stages {
		newStage, err := New(logger, stage, registerer, minStability)
		if err != nil {
			return nil, fmt.Errorf("invalid stage config %w", err)
		}
		st = append(st, newStage)
	}
	return &Pipeline{
		logger:    log.With(logger, "component", "pipeline"),
		stages:    st,
		dropCount: getDropCountMetric(registerer),
	}, nil
}

// syncChecker is an optional interface that stages with nested pipelines (e.g.
// matcherStage) can implement to report whether their internal pipeline also
// supports synchronous processing. Pipeline.CanProcessSync checks this in
// addition to the SyncStage interface so that a match stage with a non-sync
// inner pipeline does not falsely advertise sync capability.
type syncChecker interface {
	canProcessSync() bool
}

// CanProcessSync returns true when every stage in the pipeline implements
// SyncStage and, for stages with nested pipelines (match), that nested pipeline
// also supports synchronous processing.
func (p *Pipeline) CanProcessSync() bool {
	for _, s := range p.stages {
		if _, ok := s.(SyncStage); !ok {
			return false
		}
		if sc, ok := s.(syncChecker); ok && !sc.canProcessSync() {
			return false
		}
	}
	return true
}

// processEntryFull applies all pipeline stages to an already-initialized Entry,
// preserving the existing Extracted map. This is the inner loop used by
// ProcessEntry and by matcherStage.ProcessEntry for nested pipelines; it does
// NOT reinitialize Extracted from labels (unlike the public ProcessEntry).
// All stages must implement SyncStage; callers are responsible for ensuring
// CanProcessSync() is true before calling this method.
func (p *Pipeline) processEntryFull(e Entry) []Entry {
	entries := []Entry{e}
	for _, stage := range p.stages {
		if len(entries) == 0 {
			break
		}
		ss := stage.(SyncStage) // safe: caller guarantees CanProcessSync
		var next []Entry
		for _, entry := range entries {
			next = append(next, ss.ProcessEntry(entry)...)
		}
		entries = next
	}
	return entries
}

// ProcessEntry applies all stages to a single entry synchronously.
// The pipeline must support synchronous processing (CanProcessSync() == true).
// It initialises the Extracted map from the entry labels, applies each stage in
// order, and returns the resulting entries (empty = dropped, multiple = expanded).
func (p *Pipeline) ProcessEntry(e loki.Entry) []loki.Entry {
	inner := Entry{
		Extracted: make(map[string]any, len(e.Labels)),
		Entry:     e,
	}
	for k, v := range e.Labels {
		inner.Extracted[string(k)] = string(v)
	}

	processed := p.processEntryFull(inner)

	result := make([]loki.Entry, len(processed))
	for i, entry := range processed {
		result[i] = entry.Entry
	}
	return result
}

// StartSync starts the pipeline using a single goroutine instead of the N+3
// goroutine chain produced by Start. It requires CanProcessSync() == true.
//
// Resource usage comparison (N = number of stages):
//
//	Start:     N+3 goroutines (1 label-init RunWith + 1 per stage + 2 adapters)
//	StartSync: 1 goroutine
func (p *Pipeline) StartSync(in chan loki.Entry, out chan<- loki.Entry) loki.EntryHandler {
	ctx, cancel := context.WithCancel(context.Background())

	var (
		wg   sync.WaitGroup
		once sync.Once
	)

	wg.Go(func() {
		defer p.Cleanup()
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-in:
				for _, result := range p.ProcessEntry(e) {
					out <- result
				}
			}
		}
	})

	return loki.NewEntryHandler(in, func() {
		once.Do(func() { cancel() })
		wg.Wait()
	})
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
		once.Do(func() { cancel() })
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
