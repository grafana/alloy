//go:build linux && cgo && promtail_journal_enabled

package target

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/pkg/push"
	jsoniter "github.com/json-iterator/go"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

// OtelJournalTarget tails systemd journal entries using the OpenTelemetry
// journald input operator.
type OtelJournalTarget struct {
	metrics       *Metrics
	logger        log.Logger
	handler       loki.EntryHandler
	positions     positions.Positions
	positionPath  string
	relabelConfig []*relabel.Config
	config        *scrapeconfig.JournalTargetConfig
	staticLabels  model.LabelSet

	pipe      pipeline.Pipeline
	persister *positionsPersister
}

// NewOtelJournalTarget creates a new OtelJournalTarget that uses the OTEL journald operator.
func NewOtelJournalTarget(
	metrics *Metrics,
	logger log.Logger,
	handler loki.EntryHandler,
	pos positions.Positions,
	jobName string,
	relabelConfig []*relabel.Config,
	targetConfig *scrapeconfig.JournalTargetConfig,
) (*OtelJournalTarget, error) {
	positionPath := positions.CursorKey(jobName)

	t := &OtelJournalTarget{
		metrics:       metrics,
		logger:        logger,
		handler:       handler,
		positions:     pos,
		positionPath:  positionPath,
		relabelConfig: relabelConfig,
		staticLabels:  targetConfig.Labels,
		config:        targetConfig,
	}

	// Build the journald config
	journaldCfg := journald.NewConfigWithID("journald_input")

	// Map Alloy config to OTEL journald config
	if targetConfig.Path != "" {
		journaldCfg.Directory = &targetConfig.Path
	}

	// Parse matches string (format: "FIELD=value FIELD2=value2")
	if targetConfig.Matches != "" {
		matches := strings.Fields(targetConfig.Matches)
		matchConfigs := []journald.MatchConfig{}
		currentMatch := journald.MatchConfig{}

		for _, m := range matches {
			if m == "+" {
				// OR separator - start new match config
				if len(currentMatch) > 0 {
					matchConfigs = append(matchConfigs, currentMatch)
					currentMatch = journald.MatchConfig{}
				}
				continue
			}

			parts := strings.SplitN(m, "=", 2)
			if len(parts) == 2 {
				currentMatch[parts[0]] = parts[1]
			}
		}
		if len(currentMatch) > 0 {
			matchConfigs = append(matchConfigs, currentMatch)
		}
		journaldCfg.Matches = matchConfigs
	}

	// Create a zap logger from the go-kit logger
	zapLogger := createZapLogger(logger)

	// Create the output operator that will receive entries
	output := newLokiOutputOperator(t)

	// Build the pipeline
	pipeConfig := pipeline.Config{
		Operators:     []operator.Config{journaldCfg},
		DefaultOutput: output,
	}

	telemetrySettings := component.TelemetrySettings{
		Logger: zapLogger,
	}

	pipe, err := pipeConfig.Build(telemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	t.pipe = pipe

	// Create persister adapter
	t.persister = &positionsPersister{
		positions:    pos,
		positionPath: positionPath,
	}

	// Start the pipeline
	if err := t.pipe.Start(t.persister); err != nil {
		return nil, fmt.Errorf("failed to start pipeline: %w", err)
	}

	level.Info(logger).Log("msg", "started OTEL-based journal target")

	return t, nil
}

// Stop shuts down the OtelJournalTarget.
func (t *OtelJournalTarget) Stop() error {
	var err error
	if t.pipe != nil {
		err = t.pipe.Stop()
	}
	t.handler.Stop()
	return err
}

// positionsPersister implements operator.Persister using Alloy's positions system.
type positionsPersister struct {
	positions    positions.Positions
	positionPath string
	mu           sync.Mutex
}

func (p *positionsPersister) Get(_ context.Context, key string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	val := p.positions.GetString(p.positionPath, key)
	if val == "" {
		return nil, nil
	}
	return []byte(val), nil
}

func (p *positionsPersister) Set(_ context.Context, key string, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.positions.PutString(p.positionPath, key, string(value))
	return nil
}

func (p *positionsPersister) Delete(_ context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.positions.Remove(p.positionPath, key)
	return nil
}

func (p *positionsPersister) Batch(ctx context.Context, ops ...*storage.Operation) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			// Get operations don't modify state
		case storage.Set:
			p.positions.PutString(p.positionPath, op.Key, string(op.Value))
		case storage.Delete:
			p.positions.Remove(p.positionPath, op.Key)
		}
	}
	return nil
}

// lokiOutputOperator is an operator that converts OTEL entries to Loki entries.
type lokiOutputOperator struct {
	target *OtelJournalTarget
}

func newLokiOutputOperator(target *OtelJournalTarget) *lokiOutputOperator {
	return &lokiOutputOperator{target: target}
}

func (o *lokiOutputOperator) ID() string {
	return "loki_output"
}

func (o *lokiOutputOperator) Type() string {
	return "loki_output"
}

func (o *lokiOutputOperator) Start(_ operator.Persister) error {
	return nil
}

func (o *lokiOutputOperator) Stop() error {
	return nil
}

func (o *lokiOutputOperator) CanOutput() bool {
	return false
}

func (o *lokiOutputOperator) CanProcess() bool {
	return true
}

func (o *lokiOutputOperator) GetOutputIDs() []string {
	return nil
}

func (o *lokiOutputOperator) SetOutputIDs(_ []string) {}

func (o *lokiOutputOperator) Outputs() []operator.Operator {
	return nil
}

func (o *lokiOutputOperator) SetOutputs(_ []operator.Operator) error {
	return nil
}

func (o *lokiOutputOperator) Logger() *zap.Logger {
	return zap.NewNop()
}

func (o *lokiOutputOperator) Process(ctx context.Context, e *entry.Entry) error {
	o.target.processEntry(e)
	return nil
}

func (o *lokiOutputOperator) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	for _, e := range entries {
		o.target.processEntry(e)
	}
	return nil
}

// processEntry converts an OTEL entry to a Loki entry and sends it.
func (t *OtelJournalTarget) processEntry(e *entry.Entry) {
	// Build message from body
	var msg string
	if t.config.JSON {
		// For JSON mode, marshal the entire body
		if bodyMap, ok := e.Body.(map[string]any); ok {
			bb, err := marshalJSON(bodyMap)
			if err != nil {
				level.Error(t.logger).Log("msg", "could not marshal journal fields to JSON", "err", err)
				return
			}
			msg = string(bb)
		} else {
			msg = fmt.Sprintf("%v", e.Body)
		}
	} else {
		// Extract MESSAGE field from body
		if bodyMap, ok := e.Body.(map[string]any); ok {
			if msgVal, ok := bodyMap["MESSAGE"]; ok {
				switch v := msgVal.(type) {
				case string:
					msg = v
				case []byte:
					msg = string(v)
				default:
					msg = fmt.Sprintf("%v", v)
				}
			} else {
				level.Debug(t.logger).Log("msg", "received journal entry with no MESSAGE field")
				t.metrics.journalErrors.WithLabelValues(noMessageError).Inc()
				return
			}
		} else {
			msg = fmt.Sprintf("%v", e.Body)
		}
	}

	// Build entry labels from journal fields
	entryLabels := t.makeJournalFields(e.Body)

	// Add static labels
	for k, v := range t.staticLabels {
		entryLabels[string(k)] = string(v)
	}

	// Apply relabel rules
	processedLabels, _ := relabel.Process(labels.FromMap(entryLabels), t.relabelConfig...)

	processedLabelsMap := processedLabels.Map()
	lbls := make(model.LabelSet, len(processedLabelsMap))
	for k, v := range processedLabelsMap {
		if len(k) >= 2 && k[0:2] == "__" {
			continue
		}
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	if len(lbls) == 0 {
		level.Debug(t.logger).Log("msg", "received journal entry with no labels")
		t.metrics.journalErrors.WithLabelValues(emptyLabelsError).Inc()
		return
	}

	// Send entry to handler
	t.metrics.journalLines.Inc()
	t.handler.Chan() <- loki.Entry{
		Labels: lbls,
		Entry: push.Entry{
			Line:      msg,
			Timestamp: e.Timestamp,
		},
	}
}

// makeJournalFields converts journal fields to label format.
func (t *OtelJournalTarget) makeJournalFields(body any) map[string]string {
	result := make(map[string]string)

	bodyMap, ok := body.(map[string]any)
	if !ok {
		return result
	}

	for k, v := range bodyMap {
		// Skip internal fields
		if k == "__CURSOR" || k == "__REALTIME_TIMESTAMP" || k == "__MONOTONIC_TIMESTAMP" {
			continue
		}

		var strVal string
		switch val := v.(type) {
		case string:
			strVal = val
		case []byte:
			strVal = string(val)
		default:
			continue
		}

		if k == "PRIORITY" {
			result[fmt.Sprintf("__journal_%s_%s", strings.ToLower(k), "keyword")] = makeJournalPriority(strVal)
		}
		result[fmt.Sprintf("__journal_%s", strings.ToLower(k))] = strVal
	}
	return result
}

// marshalJSON marshals a map to JSON.
func marshalJSON(v any) ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(v)
}

// createZapLogger creates a zap.Logger that wraps a go-kit logger.
func createZapLogger(logger log.Logger) *zap.Logger {
	// Create a simple zap logger - in production you might want a more sophisticated bridge
	zapCfg := zap.NewProductionConfig()
	zapCfg.OutputPaths = []string{} // Disable output, we'll use the go-kit logger
	zapLogger, _ := zapCfg.Build()
	return zapLogger
}
