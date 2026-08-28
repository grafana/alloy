package diag

import (
	"io"
)

const Text = ".txt"

const criticalErrorFooter = `

A configuration file was not generated due to critical issues. Refer to the critical messages for more information.`

const errorFooter = `

A configuration file was not generated due to errors. Refer to the error messages for more information.

You can bypass the errors by using the --bypass-errors flag. Bypassing errors isn't recommended for production environments.`

const successFooter = `

A configuration file was generated successfully.`

// generateTextReport generates a text report for the diagnostics.
func generateTextReport(writer io.Writer, ds Diagnostics, bypassErrors bool) error {
	content := getContent(ds, bypassErrors)
	_, err := writer.Write([]byte(content))
	if err != nil {
		return err
	}

	return nil
}

// getContent returns the formatted content for the report based on the diagnostics and bypassErrors.
func getContent(ds Diagnostics, bypassErrors bool) string {
	var content string
	switch {
	case ds.HasSeverityLevel(SeverityLevelCritical):
		content = criticalErrorFooter
		ds.RemoveDiagsBySeverity(SeverityLevelInfo)
		ds.RemoveDiagsBySeverity(SeverityLevelWarn)
		ds.RemoveDiagsBySeverity(SeverityLevelError)
	case ds.HasSeverityLevel(SeverityLevelError) && !bypassErrors:
		content = errorFooter
		ds.RemoveDiagsBySeverity(SeverityLevelInfo)
		ds.RemoveDiagsBySeverity(SeverityLevelWarn)
	default:
		content = successFooter
	}

	return ds.Error() + content
}
