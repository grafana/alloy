// Package diag exposes error types used throughout converter and a method to
// pretty-print them to the screen.
package diag

import (
	"fmt"
)

// Diagnostic is an individual diagnostic message. Diagnostic messages can have
// different levels of severities.
type Diagnostic struct {
	// Severity holds the severity level of this Diagnostic.
	Severity Severity

	Summary string
	Detail  string
}

var _ fmt.Stringer = (*Diagnostic)(nil)

func (d Diagnostic) String() string {
	result := fmt.Sprintf("(%s) %s", d.Severity.String(), d.Summary)
	if d.Detail == "" {
		return result
	}

	return fmt.Sprintln(result) + d.Detail
}

// Error implements error.
func (d Diagnostic) Error() string {
	return d.String()
}
