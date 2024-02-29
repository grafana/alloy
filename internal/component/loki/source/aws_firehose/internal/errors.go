package internal

import "fmt"

// errWithReason implements an error type that carries a metrics-friendly reason label.
type errWithReason struct {
	// err is the original error.
	err error

	// reason provides an error cause identifier that can be used in Metrics `reason` labels.
	reason string
}

func (e errWithReason) Error() string {
	return fmt.Sprintf("%s: %s", e.reason, e.err.Error())
}

// getReason attempts to get the reason of a generic error, falling back to "unknown"
func getReason(err error) string {
	er, ok := err.(errWithReason)
	if !ok {
		return "unknown"
	}
	return er.reason
}
