package source

import (
	"time"

	"golang.org/x/time/rate"
)

var retryWarning = rate.NewLimiter(
	rate.Every(10*time.Second),
	1,
)

// AllowRetryWarning reports whether a source may log a warning about retrying.
// At most one warning is allowed every 10 seconds, shared by all sources, so a
// stalled pipeline does not flood the logs.
func AllowRetryWarning() bool {
	return retryWarning.Allow()
}
