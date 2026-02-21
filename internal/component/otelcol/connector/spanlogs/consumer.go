package spanlogs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/go-logfmt/logfmt"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.5.0"
)

const (
	typeSpan    = "span"
	typeRoot    = "root"
	typeProcess = "process"
	typeEvent   = "event"
)

type consumer struct {
	optsMut sync.RWMutex
	opts    options
}

type options struct {
	spans             bool
	roots             bool
	processes         bool
	events            bool
	spanAttributes    []string
	processAttributes []string
	eventAttributes   []string
	overrides         OverrideConfig
	labels            map[string]struct{}
	nextConsumer      otelconsumer.Logs
}

var _ otelconsumer.Traces = (*consumer)(nil)

func NewConsumer(args Arguments, nextConsumer otelconsumer.Logs) (*consumer, error) {
	c := &consumer{}

	err := c.UpdateOptions(args, nextConsumer)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *consumer) UpdateOptions(args Arguments, nextConsumer otelconsumer.Logs) error {
	c.optsMut.Lock()
	defer c.optsMut.Unlock()

	if nextConsumer == nil {
		return errors.New("nil next Consumer")
	}

	labels := make(map[string]struct{}, len(args.Labels))
	for _, l := range args.Labels {
		labels[l] = struct{}{}
	}

	c.opts = options{
		spans:             args.Spans,
		roots:             args.Roots,
		processes:         args.Processes,
		events:            args.Events,
		spanAttributes:    args.SpanAttributes,
		processAttributes: args.ProcessAttributes,
		eventAttributes:   args.EventAttributes,
		overrides:         args.Overrides,
		labels:            labels,
		nextConsumer:      nextConsumer,
	}

	return nil
}

func (c *consumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	c.optsMut.RLock()
	defer c.optsMut.RUnlock()

	// Create the logs structure which will be sent to the component's outputs
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs()

	rsLen := td.ResourceSpans().Len()
	for i := range rsLen {
		// Create a logs scope
		resLog := resourceLogs.AppendEmpty()
		scopeLogs := resLog.ScopeLogs()

		rs := td.ResourceSpans().At(i)
		ssLen := rs.ScopeSpans().Len()

		var svc string
		svcAtt, ok := rs.Resource().Attributes().Get(string(semconv.ServiceNameKey))
		if ok {
			svc = svcAtt.Str()
		}

		for j := range ssLen {
			// Create a log records slice
			scopeLog := scopeLogs.AppendEmpty()
			logRecords := scopeLog.LogRecords()

			ss := rs.ScopeSpans().At(j)

			err := c.consumeSpans(svc, ss, rs.Resource(), logRecords)
			if err != nil {
				return err
			}
		}
	}

	return c.opts.nextConsumer.ConsumeLogs(ctx, logs)
}

func (c *consumer) consumeSpans(serviceName string, ss ptrace.ScopeSpans, rs pcommon.Resource, logRecords plog.LogRecordSlice) error {
	lastTraceID := ""

	// Loop through the spans
	spanLen := ss.Spans().Len()
	for k := range spanLen {
		span := ss.Spans().At(k)
		traceID := span.TraceID().String()

		logEvents := c.opts.events
		logSpans := c.opts.spans
		logRoots := c.opts.roots && span.ParentSpanID().IsEmpty()
		logProcesses := c.opts.processes && lastTraceID != traceID

		if !logSpans && !logRoots && !logProcesses && !logEvents {
			return nil
		}

		//TODO: This code uses pcommon.Map a extensively. Should we use map[string]pcommon.Value instead?
		// It may be more efficient, because a pcommon.Map is actually just an slice.
		// Inserting to it is slow, because it has to traverse the whole slice to see if the element
		// is already in the map.
		if logProcesses {
			keyValuesProcesses := pcommon.NewMap()

			c.processKeyVals(keyValuesProcesses, rs, serviceName)

			// Add a trace ID to the key values
			keyValuesProcesses.PutStr(c.opts.overrides.TraceIDKey, traceID)

			lastTraceID = traceID
			err := c.appendLogRecord(typeProcess, keyValuesProcesses, logRecords)
			if err != nil {
				return err
			}
		}

		// Construct the key values
		keyValues := pcommon.NewMap()

		c.spanKeyVals(keyValues, span)
		c.processKeyVals(keyValues, rs, serviceName)

		// Add a trace ID to the key values
		keyValues.PutStr(c.opts.overrides.TraceIDKey, traceID)

		if logSpans {
			err := c.appendLogRecord(typeSpan, keyValues, logRecords)
			if err != nil {
				return err
			}
		}

		if logRoots {
			err := c.appendLogRecord(typeRoot, keyValues, logRecords)
			if err != nil {
				return err
			}
		}

		if logEvents {
			err := c.consumeEvents(keyValues, span.Events(), logRecords)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *consumer) consumeEvents(output pcommon.Map, events ptrace.SpanEventSlice, logRecords plog.LogRecordSlice) error {
	eventsLen := events.Len()
	for i := range eventsLen {
		event := events.At(i)

		// Can we find a solution without relying on more memory allocation?
		// Clone output map due to having multiple events in one span otherwise leading to continuous use
		// of the previous set event keyVals.
		eventOutput := pcommon.NewMap()
		output.CopyTo(eventOutput)

		c.eventKeyVals(eventOutput, event)

		err := c.appendLogRecord(typeEvent, eventOutput, logRecords)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyOtelMap(out pcommon.Map, in pcommon.Map, pred func(key string, val pcommon.Value) bool) {
	in.Range(func(k string, v pcommon.Value) bool {
		if pred(k, v) {
			newVal := out.PutEmpty(k)
			v.CopyTo(newVal)
		}
		return true
	})
}

func convertOtelMapToSlice(m pcommon.Map) []any {
	res := make([]any, 0, m.Len()*2)

	m.Range(func(k string, v pcommon.Value) bool {
		res = append(res, k)
		res = append(res, v.AsRaw())
		return true
	})

	return res
}

func (c *consumer) appendLogRecord(kind string, keyValues pcommon.Map, logRecords plog.LogRecordSlice) error {
	newLogRecord, err := c.createLogRecord(kind, keyValues)
	if err != nil {
		return err
	}

	logRecord := logRecords.AppendEmpty()
	newLogRecord.MoveTo(logRecord)
	return nil
}

func (c *consumer) createLogRecord(kind string, keyValues pcommon.Map) (*plog.LogRecord, error) {
	// Create an empty log record
	res := plog.NewLogRecord()

	// Add the log line
	keyValuesSlice := convertOtelMapToSlice(keyValues)
	logLine, err := logfmt.MarshalKeyvals(keyValuesSlice...)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal keyvals due to error: %w", err)
	}

	if logLine != nil {
		res.Body().SetStr(string(logLine))
	}

	// Add the attributes
	logAttributes := res.Attributes()

	// Add logs instance label
	logAttributes.PutStr(c.opts.overrides.LogsTag, kind)

	copyOtelMap(logAttributes, keyValues, func(key string, val pcommon.Value) bool {
		// Check if we have to include this label
		_, ok := c.opts.labels[key]
		return ok
	})

	return &res, nil
}

func (c *consumer) eventKeyVals(output pcommon.Map, event ptrace.SpanEvent) {
	etAtts := event.Attributes()

	for _, name := range c.opts.eventAttributes {
		att, ok := etAtts.Get(name)
		if ok {
			val := output.PutEmpty(name)
			att.CopyTo(val)
		}
	}
}

func (c *consumer) processKeyVals(output pcommon.Map, resource pcommon.Resource, svc string) {
	rsAtts := resource.Attributes()

	// Add an attribute with the service name
	output.PutStr(c.opts.overrides.ServiceKey, svc)

	for _, name := range c.opts.processAttributes {
		att, ok := rsAtts.Get(name)
		if ok {
			// name/key val pairs
			val := output.PutEmpty(name)
			att.CopyTo(val)
		}
	}
}

func (c *consumer) spanKeyVals(output pcommon.Map, span ptrace.Span) {
	output.PutStr(c.opts.overrides.SpanNameKey, span.Name())
	output.PutStr(c.opts.overrides.DurationKey, spanDuration(span))

	// Skip STATUS_CODE_UNSET to be less spammy
	if span.Status().Code() != ptrace.StatusCodeUnset {
		output.PutStr(c.opts.overrides.StatusKey, span.Status().Code().String())
	}

	for _, name := range c.opts.spanAttributes {
		att, ok := span.Attributes().Get(name)
		if ok {
			val := output.PutEmpty(name)
			att.CopyTo(val)
		}
	}
}

func spanDuration(span ptrace.Span) string {
	dur := int64(span.EndTimestamp() - span.StartTimestamp())
	return strconv.FormatInt(dur, 10) + "ns"
}

func (c *consumer) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{}
}
