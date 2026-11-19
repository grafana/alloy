package harness

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// SpanMatcher matches one captured span.
type SpanMatcher struct {
	match func(span Span) bool
	text  string
}

// TraceSpanCount returns an Assertion that passes when exactly want spans
// match all provided matchers. When no matchers are provided, all spans are
// counted.
func TraceSpanCount(want int, matchers ...SpanMatcher) Assertion {
	return func(s snapshot) error {
		var got int
		for _, span := range s.spans {
			if spanMatches(span, matchers...) {
				got++
			}
		}
		if got == want {
			return nil
		}

		message := fmt.Sprintf("want %d, got %d", want, got)
		if conditions := spanMatcherConditions(matchers); conditions != "" {
			message += " for " + conditions
		}
		return AssertionError{Kind: "traces.count", Message: message}
	}
}

// TraceContainsSpan returns an Assertion that passes when at least one span
// matches all provided matchers.
func TraceContainsSpan(matchers ...SpanMatcher) Assertion {
	return func(s snapshot) error {
		for _, span := range s.spans {
			if spanMatches(span, matchers...) {
				return nil
			}
		}

		message := "no matching span found"
		if conditions := spanMatcherConditions(matchers); conditions != "" {
			message += " for " + conditions
		}
		return AssertionError{Kind: "traces.contains", Message: message}
	}
}

// SpanName matches spans with the given name.
func SpanName(name string) SpanMatcher {
	return SpanMatcher{
		match: func(span Span) bool { return span.Name == name },
		text:  fmt.Sprintf("name = %q", name),
	}
}

// SpanKind matches spans of the given kind, such as Server or Client.
func SpanKind(kind string) SpanMatcher {
	return SpanMatcher{
		match: func(span Span) bool { return strings.EqualFold(span.Kind, kind) },
		text:  fmt.Sprintf("kind = %q", kind),
	}
}

// SpanStatus matches spans with the given status code: Unset, Ok or Error.
func SpanStatus(code string) SpanMatcher {
	return SpanMatcher{
		match: func(span Span) bool { return strings.EqualFold(span.StatusCode, code) },
		text:  fmt.Sprintf("status = %q", code),
	}
}

// SpanScope matches spans emitted by the given instrumentation scope.
func SpanScope(scope string) SpanMatcher {
	return SpanMatcher{
		match: func(span Span) bool { return span.Scope == scope },
		text:  fmt.Sprintf("scope = %q", scope),
	}
}

// SpanAttributes matches span attributes. When partial is true, the span may
// carry attributes beyond want.
func SpanAttributes(want map[string]string, partial bool) SpanMatcher {
	return SpanMatcher{
		match: func(span Span) bool { return stringMapMatches(span.Attributes, want, partial) },
		text:  "attributes " + renderMapMatch(want, partial),
	}
}

// SpanResourceAttributes matches resource attributes. When partial is true,
// the resource may carry attributes beyond want.
func SpanResourceAttributes(want map[string]string, partial bool) SpanMatcher {
	return SpanMatcher{
		match: func(span Span) bool { return stringMapMatches(span.ResourceAttributes, want, partial) },
		text:  "resource_attributes " + renderMapMatch(want, partial),
	}
}

func stringMapMatches(got, want map[string]string, partial bool) bool {
	if !partial && len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if gv, ok := got[k]; !ok || gv != v {
			return false
		}
	}
	return true
}

func renderMapMatch(m map[string]string, partial bool) string {
	op := "="
	if partial {
		op = "contain"
	}
	return op + " " + renderStringMap(m)
}

func renderStringMap(m map[string]string) string {
	keys := slices.Sorted(maps.Keys(m))
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, m[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func spanMatches(span Span, matchers ...SpanMatcher) bool {
	for _, m := range matchers {
		if !m.match(span) {
			return false
		}
	}
	return true
}

func spanMatcherConditions(matchers []SpanMatcher) string {
	conditions := make([]string, 0, len(matchers))
	for _, m := range matchers {
		if m.text != "" {
			conditions = append(conditions, m.text)
		}
	}
	return strings.Join(conditions, ", ")
}

func renderSpans(spans []Span) string {
	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "spans (%d):\n", len(spans))
	for _, span := range spans {
		builder.WriteString("- ")
		builder.WriteString(renderSpan(span))
		builder.WriteByte('\n')
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

func renderSpan(span Span) string {
	parts := []string{
		fmt.Sprintf("name = %q", span.Name),
		"kind = " + span.Kind,
		"status = " + span.StatusCode,
	}
	if len(span.Attributes) > 0 {
		parts = append(parts, "attributes = "+renderStringMap(span.Attributes))
	}
	if len(span.ResourceAttributes) > 0 {
		parts = append(parts, "resource_attributes = "+renderStringMap(span.ResourceAttributes))
	}
	return strings.Join(parts, ", ")
}
