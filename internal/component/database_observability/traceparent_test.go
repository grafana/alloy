package database_observability_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/component/database_observability"
)

func Test_TryExtractTraceParent(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid traceparent with single quotes",
			input:    "SELECT * FROM users /*traceparent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "valid traceparent with double quotes",
			input:    `SELECT * FROM users /*traceparent="00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"*/`,
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "valid traceparent without quotes",
			input:    "SELECT * FROM users /*traceparent=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "traceparent with mixed case keyword",
			input:    "SELECT * FROM users /*TraceParent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "traceparent among other comment fields",
			input:    "SELECT * FROM users /*controller='index',traceparent='00-abc123-def456-01',framework='django'*/",
			expected: "00-abc123-def456-01",
		},
		{
			name:     "no traceparent in SQL",
			input:    "SELECT * FROM users WHERE id = 1",
			expected: "",
		},
		{
			name:     "truncated SQL ending with ...",
			input:    "SELECT * FROM users WHERE id = 1 /*traceparent='00-abc...",
			expected: "",
		},
		{
			name:     "truncated as traceparent=... ",
			input:    "SELECT * FROM users WHERE id = 1 /*traceparent=...",
			expected: "",
		},
		{
			name:     "truncated as traceparent=",
			input:    "SELECT * FROM users WHERE id = 1 /*traceparent=",
			expected: "",
		},
		{
			name:     "traceparent without closing quote",
			input:    "SELECT * FROM users /*traceparent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			expected: "",
		},
		{
			name:     "empty traceparent value",
			input:    "SELECT * FROM users /*traceparent=''*/",
			expected: "",
		},
		{
			name:     "traceparent with whitespace",
			input:    "SELECT * FROM users /*traceparent='  00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01  '*/",
			expected: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:     "multiple traceparent occurrences - last one wins",
			input:    "SELECT * FROM users /*traceparent='00-first-first-01'*/ /*traceparent='00-second-second-02'*/",
			expected: "00-second-second-02",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name: "SQLCommenter exhibit",
			// Note that traceparent and value (W3C trace context) cannot have meta characters nor URL to decode, so they are effectively inert to TryExtractTraceParent
			input: `SELECT * FROM FOO /*action='%2Fparam*\'d',controller='index,'framework='spring',` +
				"\n" + `traceparent='00-5bd66ef5095369c7b0d1f8f4bd33716a-c532cb4098ac3dd2-01',` +
				"\n" + `tracestate='congo%3Dt61rcWkgMzE%2Crojo%3D00f067aa0ba902b7'*/`,
			expected: "00-5bd66ef5095369c7b0d1f8f4bd33716a-c532cb4098ac3dd2-01",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := database_observability.TryExtractTraceParent(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
