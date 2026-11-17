package harness

import (
	"strings"
	"time"
)

type Assertion func(s snapshot) error

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
	return strings.Join(sections, "\n\n")
}

func renderTimestamp(timestamp time.Time) string {
	return "timestamp = " + timestamp.Format(time.RFC3339Nano)
}
