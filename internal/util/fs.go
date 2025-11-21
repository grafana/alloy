package util

import (
	"errors"
	"os"
	"strings"

	"github.com/rogpeppe/go-internal/robustio"
)

// IsEphemeralOrFileClosed checks if the error is an ephemeral error or if the file is already closed. This is useful
// on certain file systems (e.g. on Windows) where in practice reading a file can result in ephemeral errors
// (e.g. due to antivirus scans) or if the file appears as closed when being removed or rotated.
func IsEphemeralOrFileClosed(err error) bool {
	return robustio.IsEphemeralError(err) ||
		errors.Is(err, os.ErrClosed) ||
		// The above errors.Is(os.ErrClosedm, err) condition doesn't always capture the 'file already closed' error on
		// Windows. Check the error message as well.
		// Inspired by https://github.com/grafana/loki/blob/987e551f9e21b9a612dd0b6a3e60503ce6fe13a8/clients/cmd/docker-driver/driver.go#L145
		strings.Contains(err.Error(), "file already closed")
}
