package diag

import (
	"io"
)

const Text = ".txt"

// generateTextReport generates a text report for the diagnostics.
func generateTextReport(writer io.Writer, ds Diagnostics) error {
	content := ds.Error()

	_, err := writer.Write([]byte(content))
	if err != nil {
		return err
	}

	return nil
}
