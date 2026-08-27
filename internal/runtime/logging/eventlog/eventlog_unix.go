//go:build !windows

package eventlog

import "errors"

var errNotSupported = errors.New("windows event log not supported on this platform")

// getEventLogOpener returns the platform's EventLogOpener (stub on non-Windows).
func GetEventLogOpener() EventLogOpener {
	return func(serviceName string) (EventLog, error) {
		return nil, errNotSupported
	}
}
