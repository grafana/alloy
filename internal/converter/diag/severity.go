// Package diag exposes error types used throughout converter and a method to
// pretty-print them to the screen.
package diag

import (
	"fmt"
)

// Severity denotes the severity level of a diagnostic. The zero value of
// severity is invalid.
type Severity int

var _ fmt.Stringer = (*Severity)(nil)

func (s Severity) String() string {
	switch s {
	case SeverityLevelCritical:
		return "Critical"
	case SeverityLevelError:
		return "Error"
	case SeverityLevelWarn:
		return "Warning"
	case SeverityLevelInfo:
		return "Info"
	default:
		return "Unknown"
	}
}

// Supported severity levels.
const (
	SeverityLevelInfo Severity = iota + 1
	SeverityLevelWarn
	SeverityLevelError
	SeverityLevelCritical
)
