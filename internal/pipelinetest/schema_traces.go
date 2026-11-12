package pipelinetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

// TraceInputSchema sends spans to Components, or, with Replace, where an
// OTel receiver such as otelcol.receiver.otlp.default sent its traces.
type TraceInputSchema struct {
	Components []string     `yaml:"components,omitempty"`
	Replace    string       `yaml:"replace,omitempty"`
	Spans      []SpanSchema `yaml:"spans"`
}

// SpanSchema is one span. Each span is sent with its own resource.
type SpanSchema struct {
	Name string `yaml:"name"`
	// ServiceName sets the service.name resource attribute.
	ServiceName        string            `yaml:"service_name,omitempty"`
	ResourceAttributes map[string]string `yaml:"resource_attributes,omitempty"`
	Attributes         map[string]string `yaml:"attributes,omitempty"`
	// Kind is internal, server, client, producer or consumer. Defaults to
	// internal.
	Kind string `yaml:"kind,omitempty"`
	// Status is unset, ok or error. Defaults to unset.
	Status string `yaml:"status,omitempty"`
	// TraceID and SpanID are hex. Random IDs are used when unset.
	TraceID      string `yaml:"trace_id,omitempty"`
	SpanID       string `yaml:"span_id,omitempty"`
	ParentSpanID string `yaml:"parent_span_id,omitempty"`
	// Duration defaults to 10ms. The span ends when it is sent.
	Duration string `yaml:"duration,omitempty"`
}

// TraceAssertionSchema describes one declarative traces assertion.
type TraceAssertionSchema struct {
	// Sink, when set, checks only what the sink replacing this exit
	// component captured, such as loki.write.audit. It must be one of
	// config.sinks.
	Sink  string           `yaml:"sink,omitempty"`
	Name  string           `yaml:"name,omitempty"`
	Type  string           `yaml:"type,omitempty"`
	Count *int             `yaml:"count,omitempty"`
	Match TraceMatchSchema `yaml:"match,omitempty"`
}

// TraceMatchSchema matches span fields. Attribute maps match partially by
// default, because SDKs and processors add their own attributes.
type TraceMatchSchema struct {
	Name string `yaml:"name,omitempty"`
	// ServiceName matches the service.name resource attribute.
	ServiceName        string         `yaml:"service_name,omitempty"`
	Kind               string         `yaml:"kind,omitempty"`
	Status             string         `yaml:"status,omitempty"`
	Scope              string         `yaml:"scope,omitempty"`
	Attributes         MapMatchSchema `yaml:"attributes,omitempty"`
	ResourceAttributes MapMatchSchema `yaml:"resource_attributes,omitempty"`
}

func buildTraceAssertions(assertions []TraceAssertionSchema) ([]harness.Assertion, error) {
	out := make([]harness.Assertion, 0, len(assertions))
	for i, assertion := range assertions {
		matchers, err := buildSpanMatchers(assertion.Match)
		if err != nil {
			return nil, fmt.Errorf("traces assertion %d: %w", i, err)
		}

		switch assertionType(assertion.Type, assertion.Count) {
		case assertionTypeCount:
			if assertion.Count == nil {
				return nil, fmt.Errorf("traces assertion %d: count requires count", i)
			}
			out = append(out, harness.TraceSpanCount(*assertion.Count, matchers...))
		case assertionTypeContains:
			if len(matchers) == 0 {
				return nil, fmt.Errorf("traces assertion %d: contains requires at least one match field", i)
			}
			out = append(out, harness.TraceContainsSpan(matchers...))
		default:
			return nil, fmt.Errorf("unknown assertion type %q", assertion.Type)
		}
	}
	return out, nil
}

func buildSpanMatchers(match TraceMatchSchema) ([]harness.SpanMatcher, error) {
	var matchers []harness.SpanMatcher
	if match.Name != "" {
		matchers = append(matchers, harness.SpanName(match.Name))
	}
	if match.Kind != "" {
		matchers = append(matchers, harness.SpanKind(match.Kind))
	}
	if match.Status != "" {
		matchers = append(matchers, harness.SpanStatus(match.Status))
	}
	if match.Scope != "" {
		matchers = append(matchers, harness.SpanScope(match.Scope))
	}
	if match.ServiceName != "" {
		matchers = append(matchers, harness.SpanResourceAttributes(map[string]string{"service.name": match.ServiceName}, true))
	}
	if len(match.Attributes.Values) > 0 {
		partial, err := isPartialMapMatchDefault("attributes", match.Attributes, true)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, harness.SpanAttributes(match.Attributes.Values, partial))
	}
	if len(match.ResourceAttributes.Values) > 0 {
		partial, err := isPartialMapMatchDefault("resource_attributes", match.ResourceAttributes, true)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, harness.SpanResourceAttributes(match.ResourceAttributes.Values, partial))
	}
	return matchers, nil
}

func buildTraces(spans []SpanSchema) (ptrace.Traces, error) {
	td := ptrace.NewTraces()
	now := time.Now()

	for i, s := range spans {
		if s.Name == "" {
			return td, fmt.Errorf("span %d requires a name", i)
		}

		rs := td.ResourceSpans().AppendEmpty()
		for k, v := range s.ResourceAttributes {
			rs.Resource().Attributes().PutStr(k, v)
		}
		if s.ServiceName != "" {
			rs.Resource().Attributes().PutStr("service.name", s.ServiceName)
		}

		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName(s.Name)
		for k, v := range s.Attributes {
			span.Attributes().PutStr(k, v)
		}

		kind, err := parseSpanKind(s.Kind)
		if err != nil {
			return td, fmt.Errorf("span %d: %w", i, err)
		}
		span.SetKind(kind)

		status, err := parseStatusCode(s.Status)
		if err != nil {
			return td, fmt.Errorf("span %d: %w", i, err)
		}
		span.Status().SetCode(status)

		traceID, err := parseTraceID(s.TraceID)
		if err != nil {
			return td, fmt.Errorf("span %d: %w", i, err)
		}
		span.SetTraceID(traceID)

		spanID, err := parseSpanID(s.SpanID, true)
		if err != nil {
			return td, fmt.Errorf("span %d: %w", i, err)
		}
		span.SetSpanID(spanID)

		parentID, err := parseSpanID(s.ParentSpanID, false)
		if err != nil {
			return td, fmt.Errorf("span %d: %w", i, err)
		}
		span.SetParentSpanID(parentID)

		duration := 10 * time.Millisecond
		if s.Duration != "" {
			duration, err = time.ParseDuration(s.Duration)
			if err != nil {
				return td, fmt.Errorf("span %d: parse duration %q: %w", i, s.Duration, err)
			}
		}
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-duration)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
	}
	return td, nil
}

func parseSpanKind(kind string) (ptrace.SpanKind, error) {
	switch strings.ToLower(kind) {
	case "", "internal":
		return ptrace.SpanKindInternal, nil
	case "server":
		return ptrace.SpanKindServer, nil
	case "client":
		return ptrace.SpanKindClient, nil
	case "producer":
		return ptrace.SpanKindProducer, nil
	case "consumer":
		return ptrace.SpanKindConsumer, nil
	default:
		return 0, fmt.Errorf("unknown span kind %q", kind)
	}
}

func parseStatusCode(status string) (ptrace.StatusCode, error) {
	switch strings.ToLower(status) {
	case "", "unset":
		return ptrace.StatusCodeUnset, nil
	case "ok":
		return ptrace.StatusCodeOk, nil
	case "error":
		return ptrace.StatusCodeError, nil
	default:
		return 0, fmt.Errorf("unknown span status %q", status)
	}
}

func parseTraceID(s string) (pcommon.TraceID, error) {
	var id pcommon.TraceID
	if s == "" {
		_, _ = rand.Read(id[:])
		return id, nil
	}
	if err := decodeHexID(s, id[:]); err != nil {
		return id, fmt.Errorf("trace_id: %w", err)
	}
	return id, nil
}

func parseSpanID(s string, random bool) (pcommon.SpanID, error) {
	var id pcommon.SpanID
	if s == "" {
		if random {
			_, _ = rand.Read(id[:])
		}
		return id, nil
	}
	if err := decodeHexID(s, id[:]); err != nil {
		return id, fmt.Errorf("span id: %w", err)
	}
	return id, nil
}

func decodeHexID(s string, dst []byte) error {
	b, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("%q is not hex: %w", s, err)
	}
	if len(b) != len(dst) {
		return fmt.Errorf("%q must be %d hex characters", s, len(dst)*2)
	}
	copy(dst, b)
	return nil
}

func produceTraceInputs(ctx context.Context, alloy *harness.Alloy, ids inputComponents, inputs []TraceInputSchema) error {
	for i, input := range inputs {
		if len(input.Spans) == 0 {
			continue
		}
		td, err := buildTraces(input.Spans)
		if err != nil {
			return fmt.Errorf("traces input %d: %w", i, err)
		}
		sources, err := ids.sources(alloy, signalTraces, i)
		if err != nil {
			return err
		}
		for _, source := range sources {
			if err := source.SendTraces(ctx, td); err != nil {
				return fmt.Errorf("traces input %d: %w", i, err)
			}
		}
	}
	return nil
}
