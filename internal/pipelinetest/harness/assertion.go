package harness

import (
	"fmt"
	"strings"
	"time"
)

type Assertion func(s snapshot) error

// ForSink returns an Assertion that checks only what the named sink captured.
func ForSink(name string, a Assertion) Assertion {
	return func(s snapshot) error {
		sub, ok := s.bySink[name]
		if !ok {
			return AssertionError{Kind: "sink", Message: fmt.Sprintf("no sink named %s", name)}
		}
		sub.targets = s.targets
		return a(sub)
	}
}

type AssertionError struct {
	Kind    string
	Message string
}

func (e AssertionError) Error() string {
	if e.Kind == "" {
		return e.Message
	}
	return e.Kind + ": " + e.Message
}

type AssertionErrors struct {
	Errors   []error
	Snapshot snapshot
}

func (e AssertionErrors) Error() string {
	var builder strings.Builder
	builder.WriteString("pipeline test failed\n\n")

	for _, err := range e.Errors {
		builder.WriteString("- ")
		builder.WriteString(err.Error())
		builder.WriteByte('\n')
	}

	builder.WriteString("\nlatest snapshot:\n")
	builder.WriteString(renderSnapshot(e.Snapshot))

	return strings.TrimSuffix(builder.String(), "\n")
}

func renderSnapshot(s snapshot) string {
	sections := []string{
		renderLokiEntries(s.loki),
		renderPrometheusSamples(s.prometheus),
	}
	if len(s.spans) > 0 {
		sections = append(sections, renderSpans(s.spans))
	}
	return strings.Join(sections, "\n\n")
}

func renderTimestamp(timestamp time.Time) string {
	return "timestamp = " + timestamp.Format(time.RFC3339Nano)
}
