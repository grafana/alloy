package diag

import (
	"testing"

	"github.com/grafana/alloy/syntax/token"
	"github.com/stretchr/testify/assert"
)

func TestDiagnostic_Error(t *testing.T) {
	d := Diagnostic{
		Severity: SeverityLevelError,
		StartPos: token.Position{
			Filename: "test.alloy",
			Line:     5,
			Column:   10,
		},
		Message: "unexpected token",
		Value:   "invalid",
	}

	expected := "test.alloy:5:10: unexpected token"
	assert.Equal(t, expected, d.Error())
}

func TestDiagnostics_Add(t *testing.T) {
	var ds Diagnostics

	d1 := Diagnostic{
		Severity: SeverityLevelError,
		StartPos: token.Position{Filename: "test1.alloy", Line: 1, Column: 1},
		Message:  "error 1",
	}

	d2 := Diagnostic{
		Severity: SeverityLevelWarn,
		StartPos: token.Position{Filename: "test2.alloy", Line: 2, Column: 2},
		Message:  "warning 1",
	}

	ds.Add(d1)
	assert.Len(t, ds, 1)
	assert.Equal(t, d1, ds[0])

	ds.Add(d2)
	assert.Len(t, ds, 2)
	assert.Equal(t, d2, ds[1])
}

func TestDiagnostics_Merge(t *testing.T) {
	d1 := Diagnostic{
		Severity: SeverityLevelError,
		StartPos: token.Position{Filename: "test1.alloy", Line: 1, Column: 1},
		Message:  "error 1",
	}

	d2 := Diagnostic{
		Severity: SeverityLevelWarn,
		StartPos: token.Position{Filename: "test2.alloy", Line: 2, Column: 2},
		Message:  "warning 1",
	}

	d3 := Diagnostic{
		Severity: SeverityLevelError,
		StartPos: token.Position{Filename: "test3.alloy", Line: 3, Column: 3},
		Message:  "error 2",
	}

	ds1 := Diagnostics{d1, d2}
	ds2 := Diagnostics{d3}

	ds1.Merge(ds2)
	assert.Len(t, ds1, 3)
	assert.Equal(t, d1, ds1[0])
	assert.Equal(t, d2, ds1[1])
	assert.Equal(t, d3, ds1[2])
}

func TestDiagnostics_Error(t *testing.T) {
	tests := []struct {
		name        string
		diagnostics Diagnostics
		expected    string
	}{
		{
			name:        "empty diagnostics",
			diagnostics: Diagnostics{},
			expected:    "no errors",
		},
		{
			name: "single diagnostic",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
					Message:  "single error",
				},
			},
			expected: "test.alloy:1:1: single error",
		},
		{
			name: "multiple diagnostics",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
					Message:  "first error",
				},
				{
					Severity: SeverityLevelWarn,
					StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 1},
					Message:  "warning",
				},
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 3, Column: 1},
					Message:  "second error",
				},
			},
			expected: "test.alloy:1:1: first error (and 2 more diagnostics)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.diagnostics.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiagnostics_ErrorOrNil(t *testing.T) {
	// Test empty diagnostics
	var ds Diagnostics
	err := ds.ErrorOrNil()
	assert.Nil(t, err)

	// Test non-empty diagnostics
	ds.Add(Diagnostic{
		Severity: SeverityLevelError,
		StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
		Message:  "test error",
	})
	err = ds.ErrorOrNil()
	assert.NotNil(t, err)
	assert.Equal(t, ds, err)
}

func TestDiagnostics_HasErrors(t *testing.T) {
	tests := []struct {
		name        string
		diagnostics Diagnostics
		expected    bool
	}{
		{
			name:        "empty diagnostics",
			diagnostics: Diagnostics{},
			expected:    false,
		},
		{
			name: "only warnings",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelWarn,
					StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
					Message:  "warning 1",
				},
				{
					Severity: SeverityLevelWarn,
					StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 1},
					Message:  "warning 2",
				},
			},
			expected: false,
		},
		{
			name: "mixed warnings and errors",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelWarn,
					StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
					Message:  "warning",
				},
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 1},
					Message:  "error",
				},
			},
			expected: true,
		},
		{
			name: "only errors",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
					Message:  "error 1",
				},
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 1},
					Message:  "error 2",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.diagnostics.HasErrors()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiagnostics_AllMessages(t *testing.T) {
	tests := []struct {
		name        string
		diagnostics Diagnostics
		expected    string
	}{
		{
			name:        "empty diagnostics",
			diagnostics: Diagnostics{},
			expected:    "no errors",
		},
		{
			name: "single diagnostic",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
					Message:  "single error",
				},
			},
			expected: "test.alloy:1:1: single error",
		},
		{
			name: "multiple diagnostics",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
					Message:  "first error",
				},
				{
					Severity: SeverityLevelWarn,
					StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 5},
					Message:  "warning message",
				},
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "test.alloy", Line: 3, Column: 10},
					Message:  "second error",
				},
			},
			expected: "test.alloy:1:1: first error; test.alloy:2:5: warning message; test.alloy:3:10: second error",
		},
		{
			name: "many diagnostics",
			diagnostics: Diagnostics{
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "config.alloy", Line: 5, Column: 10},
					Message:  "syntax error: unexpected token",
				},
				{
					Severity: SeverityLevelWarn,
					StartPos: token.Position{Filename: "config.alloy", Line: 8, Column: 15},
					Message:  "deprecated field usage",
				},
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "config.alloy", Line: 12, Column: 1},
					Message:  "undefined variable 'foo'",
				},
				{
					Severity: SeverityLevelError,
					StartPos: token.Position{Filename: "other.alloy", Line: 2, Column: 3},
					Message:  "type mismatch",
				},
			},
			expected: "config.alloy:5:10: syntax error: unexpected token; config.alloy:8:15: deprecated field usage; config.alloy:12:1: undefined variable 'foo'; other.alloy:2:3: type mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.diagnostics.AllMessages()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiagnostics_AllMessages_vs_Error(t *testing.T) {
	// Test the key difference between AllMessages() and Error()
	// Error() truncates after the first diagnostic when there are multiple
	// AllMessages() includes all diagnostics
	diagnostics := Diagnostics{
		{
			Severity: SeverityLevelError,
			StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
			Message:  "first error",
		},
		{
			Severity: SeverityLevelError,
			StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 1},
			Message:  "second error",
		},
		{
			Severity: SeverityLevelWarn,
			StartPos: token.Position{Filename: "test.alloy", Line: 3, Column: 1},
			Message:  "warning",
		},
	}

	errorResult := diagnostics.Error()
	allMessagesResult := diagnostics.AllMessages()

	// Error() should be truncated
	expectedError := "test.alloy:1:1: first error (and 2 more diagnostics)"
	assert.Equal(t, expectedError, errorResult)

	// AllMessages() should include all
	expectedAllMessages := "test.alloy:1:1: first error; test.alloy:2:1: second error; test.alloy:3:1: warning"
	assert.Equal(t, expectedAllMessages, allMessagesResult)

	// Verify they're different when there are multiple diagnostics
	assert.NotEqual(t, errorResult, allMessagesResult)
}

func TestSeverityLevels(t *testing.T) {
	// Test that severity levels are distinct
	assert.NotEqual(t, SeverityLevelWarn, SeverityLevelError)
	assert.Equal(t, Severity(1), SeverityLevelWarn)
	assert.Equal(t, Severity(2), SeverityLevelError)
}

func TestDiagnostic_WithEndPos(t *testing.T) {
	d := Diagnostic{
		Severity: SeverityLevelError,
		StartPos: token.Position{
			Filename: "test.alloy",
			Line:     5,
			Column:   10,
		},
		EndPos: token.Position{
			Filename: "test.alloy",
			Line:     5,
			Column:   15,
		},
		Message: "invalid range",
		Value:   "some_value",
	}

	// Test that EndPos is preserved
	assert.Equal(t, "test.alloy", d.EndPos.Filename)
	assert.Equal(t, 5, d.EndPos.Line)
	assert.Equal(t, 15, d.EndPos.Column)

	// Error should still use StartPos
	expected := "test.alloy:5:10: invalid range"
	assert.Equal(t, expected, d.Error())
}
