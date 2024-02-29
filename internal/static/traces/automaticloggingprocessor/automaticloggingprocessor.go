package automaticloggingprocessor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-logfmt/logfmt"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/agent/internal/static/operator/config"
	"github.com/grafana/agent/internal/static/traces/contextkeys"
	util "github.com/grafana/agent/internal/util/log"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/atomic"
)

const (
	defaultLogsTag     = "traces"
	defaultServiceKey  = "svc"
	defaultSpanNameKey = "span"
	defaultStatusKey   = "status"
	defaultDurationKey = "dur"
	defaultTraceIDKey  = "tid"

	defaultTimeout = time.Millisecond

	typeSpan    = "span"
	typeRoot    = "root"
	typeProcess = "process"
)

type automaticLoggingProcessor struct {
	nextConsumer consumer.Traces

	cfg          *AutomaticLoggingConfig
	logToStdout  bool
	logsInstance *logs.Instance
	done         atomic.Bool

	labels map[string]struct{}

	logger log.Logger
}

func newTraceProcessor(nextConsumer consumer.Traces, cfg *AutomaticLoggingConfig) (processor.Traces, error) {
	logger := log.With(util.Logger, "component", "traces automatic logging")

	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	if !cfg.Roots && !cfg.Processes && !cfg.Spans {
		return nil, errors.New("automaticLoggingProcessor requires one of roots, processes, or spans to be enabled")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}

	if cfg.Backend == "" {
		cfg.Backend = BackendStdout
	}

	if cfg.Backend != BackendLogs && cfg.Backend != BackendStdout {
		return nil, fmt.Errorf("automaticLoggingProcessor requires a backend of type '%s' or '%s'", BackendLogs, BackendStdout)
	}

	logToStdout := false
	if cfg.Backend == BackendStdout {
		logToStdout = true
	}

	cfg.Overrides.LogsTag = override(cfg.Overrides.LogsTag, defaultLogsTag)
	cfg.Overrides.ServiceKey = override(cfg.Overrides.ServiceKey, defaultServiceKey)
	cfg.Overrides.SpanNameKey = override(cfg.Overrides.SpanNameKey, defaultSpanNameKey)
	cfg.Overrides.StatusKey = override(cfg.Overrides.StatusKey, defaultStatusKey)
	cfg.Overrides.DurationKey = override(cfg.Overrides.DurationKey, defaultDurationKey)
	cfg.Overrides.TraceIDKey = override(cfg.Overrides.TraceIDKey, defaultTraceIDKey)

	labels := make(map[string]struct{}, len(cfg.Labels))
	for _, l := range cfg.Labels {
		labels[l] = struct{}{}
	}

	return &automaticLoggingProcessor{
		nextConsumer: nextConsumer,
		cfg:          cfg,
		logToStdout:  logToStdout,
		logger:       logger,
		done:         atomic.Bool{},
		labels:       labels,
	}, nil
}

func (p *automaticLoggingProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rsLen := td.ResourceSpans().Len()
	for i := 0; i < rsLen; i++ {
		rs := td.ResourceSpans().At(i)
		ssLen := rs.ScopeSpans().Len()

		var svc string
		svcAtt, ok := rs.Resource().Attributes().Get(semconv.AttributeServiceName)
		if ok {
			svc = svcAtt.Str()
		}

		for j := 0; j < ssLen; j++ {
			ss := rs.ScopeSpans().At(j)
			spanLen := ss.Spans().Len()

			lastTraceID := ""
			for k := 0; k < spanLen; k++ {
				span := ss.Spans().At(k)
				traceID := span.TraceID().String()

				if p.cfg.Spans {
					keyValues := append(p.spanKeyVals(span), p.processKeyVals(rs.Resource(), svc)...)
					p.exportToLogsInstance(typeSpan, traceID, p.spanLabels(keyValues), keyValues...)
				}

				if p.cfg.Roots && span.ParentSpanID().IsEmpty() {
					keyValues := append(p.spanKeyVals(span), p.processKeyVals(rs.Resource(), svc)...)
					p.exportToLogsInstance(typeRoot, traceID, p.spanLabels(keyValues), keyValues...)
				}

				if p.cfg.Processes && lastTraceID != traceID {
					lastTraceID = traceID
					keyValues := p.processKeyVals(rs.Resource(), svc)
					p.exportToLogsInstance(typeProcess, traceID, p.spanLabels(keyValues), keyValues...)
				}
			}
		}
	}

	return p.nextConsumer.ConsumeTraces(ctx, td)
}

func (p *automaticLoggingProcessor) spanLabels(keyValues []interface{}) model.LabelSet {
	if len(keyValues) == 0 {
		return model.LabelSet{}
	}
	ls := make(map[model.LabelName]model.LabelValue, len(keyValues)/2)
	var (
		k, v string
		ok   bool
	)
	for i := 0; i < len(keyValues); i += 2 {
		if k, ok = keyValues[i].(string); !ok {
			// Should never happen, all keys are strings
			level.Error(p.logger).Log("msg", "error casting label key to string", "key", keyValues[i])
			continue
		}
		// Try to cast value to string
		if v, ok = keyValues[i+1].(string); !ok {
			// If it's not a string, format it to its string representation
			v = fmt.Sprintf("%v", keyValues[i+1])
		}
		if _, ok := p.labels[k]; ok {
			// Loki does not accept "." as a valid character for labels
			// Dots . are replaced by underscores _
			k = config.SanitizeLabelName(k)

			ls[model.LabelName(k)] = model.LabelValue(v)
		}
	}
	return ls
}

func (p *automaticLoggingProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

// Start is invoked during service startup.
func (p *automaticLoggingProcessor) Start(ctx context.Context, _ component.Host) error {
	if !p.logToStdout {
		logs, ok := ctx.Value(contextkeys.Logs).(*logs.Logs)
		if !ok {
			return fmt.Errorf("key does not contain a logs instance")
		}
		p.logsInstance = logs.Instance(p.cfg.LogsName)
		if p.logsInstance == nil {
			return fmt.Errorf("logs instance %s not found", p.cfg.LogsName)
		}
	}
	return nil
}

// Shutdown is invoked during service shutdown.
func (p *automaticLoggingProcessor) Shutdown(context.Context) error {
	p.done.Store(true)

	return nil
}

func (p *automaticLoggingProcessor) processKeyVals(resource pcommon.Resource, svc string) []interface{} {
	atts := make([]interface{}, 0, 2) // 2 for service name
	rsAtts := resource.Attributes()

	// Add an attribute with the service name
	atts = append(atts, p.cfg.Overrides.ServiceKey)
	atts = append(atts, svc)

	for _, name := range p.cfg.ProcessAttributes {
		att, ok := rsAtts.Get(name)
		if ok {
			// name/key val pairs
			atts = append(atts, name)
			atts = append(atts, attributeValue(att))
		}
	}

	return atts
}

func (p *automaticLoggingProcessor) spanKeyVals(span ptrace.Span) []interface{} {
	atts := make([]interface{}, 0, 8) // 8 for name, duration, service name and status

	atts = append(atts, p.cfg.Overrides.SpanNameKey)
	atts = append(atts, span.Name())

	atts = append(atts, p.cfg.Overrides.DurationKey)
	atts = append(atts, spanDuration(span))

	// Skip STATUS_CODE_UNSET to be less spammy
	if span.Status().Code() != ptrace.StatusCodeUnset {
		atts = append(atts, p.cfg.Overrides.StatusKey)
		atts = append(atts, span.Status().Code())
	}

	for _, name := range p.cfg.SpanAttributes {
		att, ok := span.Attributes().Get(name)
		if ok {
			atts = append(atts, name)
			atts = append(atts, attributeValue(att))
		}
	}

	return atts
}

func (p *automaticLoggingProcessor) exportToLogsInstance(kind string, traceID string, labels model.LabelSet, keyvals ...interface{}) {
	if p.done.Load() {
		return
	}

	keyvals = append(keyvals, []interface{}{p.cfg.Overrides.TraceIDKey, traceID}...)
	line, err := logfmt.MarshalKeyvals(keyvals...)
	if err != nil {
		level.Warn(p.logger).Log("msg", "unable to marshal keyvals", "err", err)
		return
	}

	// if we're logging to stdout, log and bail
	if p.logToStdout {
		level.Info(p.logger).Log(keyvals...)
		return
	}

	// Add logs instance label
	labels[model.LabelName(p.cfg.Overrides.LogsTag)] = model.LabelValue(kind)

	sent := p.logsInstance.SendEntry(api.Entry{
		Labels: labels,
		Entry: logproto.Entry{
			Timestamp: time.Now(),
			Line:      string(line),
		},
	}, p.cfg.Timeout)

	if !sent {
		level.Warn(p.logger).Log("msg", "failed to autolog to logs pipeline", "kind", kind, "traceid", traceID)
	}
}

func spanDuration(span ptrace.Span) string {
	dur := int64(span.EndTimestamp() - span.StartTimestamp())
	return strconv.FormatInt(dur, 10) + "ns"
}

func attributeValue(att pcommon.Value) interface{} {
	switch att.Type() {
	case pcommon.ValueTypeStr:
		return att.Str()
	case pcommon.ValueTypeInt:
		return att.Int()
	case pcommon.ValueTypeDouble:
		return att.Double()
	case pcommon.ValueTypeBool:
		return att.Bool()
	case pcommon.ValueTypeMap:
		return att.Map()
	case pcommon.ValueTypeSlice:
		return att.Slice()
	}
	return nil
}

func override(cfgValue string, defaultValue string) string {
	if cfgValue == "" {
		return defaultValue
	}
	return cfgValue
}
