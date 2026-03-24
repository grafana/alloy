package diag

import (
	"errors"
	"testing"

	"github.com/grafana/alloy/syntax/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions to reduce code duplication
func newTestDiagnostic(severity Severity, filename string, line, column int, message string) Diagnostic {
	return Diagnostic{
		Severity: severity,
		StartPos: token.Position{
			Filename: filename,
			Line:     line,
			Column:   column,
		},
		Message: message,
	}
}

func newTestDiagnosticWithEnd(severity Severity, filename string, startLine, startCol, endLine, endCol int, message string) Diagnostic {
	return Diagnostic{
		Severity: severity,
		StartPos: token.Position{
			Filename: filename,
			Line:     startLine,
			Column:   startCol,
		},
		EndPos: token.Position{
			Filename: filename,
			Line:     endLine,
			Column:   endCol,
		},
		Message: message,
	}
}

func TestDiagnostic_As(t *testing.T) {
	d := newTestDiagnostic(SeverityLevelError, "test.alloy", 1, 1, "test error")

	t.Run("convert to Diagnostics", func(t *testing.T) {
		var diags Diagnostics
		ok := d.As(&diags)
		assert.True(t, ok)
		assert.Len(t, diags, 1)
		assert.Equal(t, d, diags[0])
	})

	t.Run("unsupported type conversion", func(t *testing.T) {
		var str string
		ok := d.As(&str)
		assert.False(t, ok)
	})

	t.Run("nil pointer", func(t *testing.T) {
		var diags *Diagnostics
		ok := d.As(diags)
		assert.False(t, ok)
	})
}

func TestDiagnostic_Error(t *testing.T) {
	tests := []struct {
		name     string
		diag     Diagnostic
		expected string
	}{
		{
			name:     "standard error message",
			diag:     newTestDiagnostic(SeverityLevelError, "test.alloy", 5, 10, "unexpected token"),
			expected: "test.alloy:5:10: unexpected token",
		},
		{
			name:     "warning message",
			diag:     newTestDiagnostic(SeverityLevelWarn, "config.alloy", 1, 1, "deprecated field"),
			expected: "config.alloy:1:1: deprecated field",
		},
		{
			name:     "empty filename",
			diag:     newTestDiagnostic(SeverityLevelError, "", 1, 1, "error message"),
			expected: "1:1: error message",
		},
		{
			name:     "zero line/column",
			diag:     newTestDiagnostic(SeverityLevelError, "test.alloy", 0, 0, "zero position"),
			expected: "test.alloy: zero position",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.diag.Error())
		})
	}
}

func TestDiagnostics_Add(t *testing.T) {
	var ds Diagnostics

	d1 := newTestDiagnostic(SeverityLevelError, "test1.alloy", 1, 1, "error 1")
	d2 := newTestDiagnostic(SeverityLevelWarn, "test2.alloy", 2, 2, "warning 1")

	ds.Add(d1)
	assert.Len(t, ds, 1)
	assert.Equal(t, d1, ds[0])

	ds.Add(d2)
	assert.Len(t, ds, 2)
	assert.Equal(t, d2, ds[1])
}

func TestDiagnostics_Add_EdgeCases(t *testing.T) {
	t.Run("add to nil diagnostics", func(t *testing.T) {
		var ds *Diagnostics
		require.Nil(t, ds)
		// This would panic - but that's expected behavior, documenting it
	})

	t.Run("add zero value diagnostic", func(t *testing.T) {
		var ds Diagnostics
		var d Diagnostic // zero value

		ds.Add(d)
		assert.Len(t, ds, 1)
		assert.Equal(t, d, ds[0])
		assert.Equal(t, Severity(0), ds[0].Severity) // zero severity
	})
}

func TestDiagnostics_Merge(t *testing.T) {
	d1 := newTestDiagnostic(SeverityLevelError, "test1.alloy", 1, 1, "error 1")
	d2 := newTestDiagnostic(SeverityLevelWarn, "test2.alloy", 2, 2, "warning 1")
	d3 := newTestDiagnostic(SeverityLevelError, "test3.alloy", 3, 3, "error 2")

	tests := []struct {
		name     string
		initial  Diagnostics
		toMerge  Diagnostics
		expected Diagnostics
	}{
		{
			name:     "merge non-empty with non-empty",
			initial:  Diagnostics{d1, d2},
			toMerge:  Diagnostics{d3},
			expected: Diagnostics{d1, d2, d3},
		},
		{
			name:     "merge empty with non-empty",
			initial:  Diagnostics{},
			toMerge:  Diagnostics{d1, d2},
			expected: Diagnostics{d1, d2},
		},
		{
			name:     "merge non-empty with empty",
			initial:  Diagnostics{d1},
			toMerge:  Diagnostics{},
			expected: Diagnostics{d1},
		},
		{
			name:     "merge empty with empty",
			initial:  Diagnostics{},
			toMerge:  Diagnostics{},
			expected: Diagnostics{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initial := make(Diagnostics, len(tt.initial))
			copy(initial, tt.initial)

			initial.Merge(tt.toMerge)
			assert.Equal(t, tt.expected, initial)
		})
	}
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

func TestDiagnostic_WithEndPos(t *testing.T) {
	d := newTestDiagnosticWithEnd(SeverityLevelError, "test.alloy", 5, 10, 5, 15, "invalid range")
	d.Value = "some_value"

	// Test that EndPos is preserved
	assert.Equal(t, "test.alloy", d.EndPos.Filename)
	assert.Equal(t, 5, d.EndPos.Line)
	assert.Equal(t, 15, d.EndPos.Column)

	// Error should still use StartPos
	expected := "test.alloy:5:10: invalid range"
	assert.Equal(t, expected, d.Error())
}

func TestDiagnostics_ErrorOrNil_Interface(t *testing.T) {
	// Test that ErrorOrNil returns a proper error interface
	ds := Diagnostics{newTestDiagnostic(SeverityLevelError, "test.alloy", 1, 1, "test error")}

	err := ds.ErrorOrNil()
	require.NotNil(t, err)

	// Test that it can be unwrapped back to Diagnostics
	var diags Diagnostics
	ok := errors.As(err, &diags)
	assert.True(t, ok)
	assert.Equal(t, ds, diags)
}
