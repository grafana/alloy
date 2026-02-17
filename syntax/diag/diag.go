// Package diag exposes error types used throughout Alloy and a method to
// pretty-print them to the screen.
package diag

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/syntax/token"
)

// Severity denotes the severity level of a diagnostic. The zero value of
// severity is invalid.
type Severity int

// Supported severity levels.
const (
	SeverityLevelWarn Severity = iota + 1
	SeverityLevelError
)

// Diagnostic is an individual diagnostic message. Diagnostic messages can have
// different levels of severities.
type Diagnostic struct {
	// Severity holds the severity level of this Diagnostic.
	Severity Severity

	// StartPos refers to a position in a file where this Diagnostic starts.
	StartPos token.Position

	// EndPos refers to an optional position in a file where this Diagnostic
	// ends. If EndPos is the zero value, the Diagnostic should be treated as
	// only covering a single character (i.e., StartPos == EndPos).
	//
	// When defined, EndPos must have the same Filename value as the StartPos.
	EndPos token.Position

	Message string
	Value   string
}

// As allows d to be interpreted as a list of Diagnostics.
func (d Diagnostic) As(v any) bool {
	switch v := v.(type) {
	case *Diagnostics:
		if v == nil {
			return false
		}
		*v = Diagnostics{d}
		return true
	}

	return false
}

// Error implements error.
func (d Diagnostic) Error() string {
	return fmt.Sprintf("%s: %s", d.StartPos, d.Message)
}

// Diagnostics is a collection of diagnostic messages.
type Diagnostics []Diagnostic

// Add adds an individual Diagnostic to the diagnostics list.
func (ds *Diagnostics) Add(d Diagnostic) {
	*ds = append(*ds, d)
}

// Merge merges other into the diagnostics list.
func (ds *Diagnostics) Merge(other Diagnostics) {
	*ds = append(*ds, other...)
}

// Error implements error.
func (ds Diagnostics) Error() string {
	switch len(ds) {
	case 0:
		return "no errors"
	case 1:
		return ds[0].Error()
	default:
		return fmt.Sprintf("%s (and %d more diagnostics)", ds[0], len(ds)-1)
	}
}

// ErrorOrNil returns an error interface if the list diagnostics is non-empty,
// nil otherwise.
func (ds Diagnostics) ErrorOrNil() error {
	if len(ds) == 0 {
		return nil
	}
	return ds
}

// HasErrors reports whether the list of Diagnostics contain any error-level
// diagnostic.
func (ds Diagnostics) HasErrors() bool {
	for _, d := range ds {
		if d.Severity == SeverityLevelError {
			return true
		}
	}
	return false
}

// AllMessages returns a string containing all diagnostic messages,
// providing more detail than the default Error() method which truncates.
func (ds Diagnostics) AllMessages() string {
	if len(ds) == 0 {
		return "no errors"
	}

	var messages []string
	for _, d := range ds {
		messages = append(messages, d.Error())
	}
	return strings.Join(messages, "; ")
}
