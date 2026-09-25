package source

import "errors"

// Failure reasons. They are the values of the reason label on
// infinity_source_poll_failures_total.
const (
	reasonRequest     = "request"
	reasonTimeout     = "timeout"
	reasonStatus      = "status"
	reasonTooLarge    = "too_large"
	reasonParse       = "parse"
	reasonPostprocess = "postprocess"
	reasonSeriesLimit = "series_limit"
	reasonEntryLimit  = "entry_limit"
	reasonEmit        = "emit"
)

type pollError struct {
	reason string
	err    error
}

func (e *pollError) Error() string { return e.reason + ": " + e.err.Error() }
func (e *pollError) Unwrap() error { return e.err }

func newPollError(reason string, err error) error {
	return &pollError{reason: reason, err: err}
}

// reasonOf returns the failure reason of err. An error without a reason is a
// request error.
func reasonOf(err error) string {
	var pe *pollError
	if errors.As(err, &pe) {
		return pe.reason
	}
	return reasonRequest
}
