// Package diag exposes error types used throughout converter and a method to
// pretty-print them to the screen.
package diag

import (
	"fmt"
	"io"
	"strings"
)

// Diagnostics is a collection of diagnostic messages.
type Diagnostics []Diagnostic

// Add adds an individual Diagnostic to the diagnostics list.
func (ds *Diagnostics) Add(severity Severity, message string) {
	*ds = append(*ds, Diagnostic{
		Severity: severity,
		Summary:  message,
	})
}

// Add adds an individual Diagnostic to the diagnostics list.
func (ds *Diagnostics) AddWithDetail(severity Severity, message string, detail string) {
	*ds = append(*ds, Diagnostic{
		Severity: severity,
		Summary:  message,
		Detail:   detail,
	})
}

// AddAll adds all given diagnostics to the diagnostics list.
func (ds *Diagnostics) AddAll(diags Diagnostics) {
	*ds = append(*ds, diags...)
}

// Error implements error.
func (ds Diagnostics) Error() string {
	var sb strings.Builder
	for ix, diag := range ds {
		fmt.Fprint(&sb, diag.Error())
		if ix+1 < len(ds) {
			fmt.Fprintln(&sb)
		}
	}

	return sb.String()
}

func (ds Diagnostics) GenerateReport(writer io.Writer, reportType string) error {
	switch reportType {
	case Text:
		return generateTextReport(writer, ds)
	default:
		return fmt.Errorf("Invalid diagnostic report type %q", reportType)
	}
}

func (ds *Diagnostics) RemoveDiagsBySeverity(severity Severity) {
	var newDiags Diagnostics

	for _, diag := range *ds {
		if diag.Severity != severity {
			newDiags = append(newDiags, diag)
		}
	}

	*ds = newDiags
}
