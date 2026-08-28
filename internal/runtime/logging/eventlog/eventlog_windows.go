//go:build windows

package eventlog

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/eventlog"
)

const eventLogAppRegPath = `SYSTEM\CurrentControlSet\Services\EventLog\Application\`

// GetEventLogOpener returns the platform's EventLogOpener (real on Windows, stub elsewhere).
func GetEventLogOpener() EventLogOpener {
	return openEventLogWindows
}

// realEventLog wraps the Windows event log and implements EventLog.
type realEventLog struct {
	el *eventlog.Log
}

var _ EventLog = (*realEventLog)(nil)

func (r *realEventLog) Info(id uint32, msg string) error    { return r.el.Info(id, msg) }
func (r *realEventLog) Warning(id uint32, msg string) error { return r.el.Warning(id, msg) }
func (r *realEventLog) Error(id uint32, msg string) error   { return r.el.Error(id, msg) }
func (r *realEventLog) Close() error {
	if r.el != nil {
		return r.el.Close()
	}
	return nil
}

// isEventSourceRegistered reports whether the named event source has a
// registry entry under HKLM\...\EventLog\Application. Uses read-only
// QUERY_VALUE access on the subkey, which is granted to all users by
// default — no admin rights required. A false return means "not present,
// or we can't tell"; the caller then falls through to
// InstallAsEventCreate, which surfaces any deeper permission issue.
func isEventSourceRegistered(serviceName string) bool {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		eventLogAppRegPath+serviceName,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}
	_ = key.Close()
	return true
}

// openEventLogWindows installs the event source (if needed) and opens the event log.
// It is the default EventLogOpener on Windows.
//
// Registering a new event source writes to HKLM and requires administrator
// rights. To let non-admin runs proceed when the source is already
// registered, we probe the registry read-only first and skip the install
// entirely on a hit.
//
// TODO: Also do the registration in the Alloy Windows installer.
// That way users don't have to run Alloy with admin rights one time just for this to be installed.
func openEventLogWindows(serviceName string) (EventLog, error) {
	if !isEventSourceRegistered(serviceName) {
		eventTypes := uint32(eventlog.Info | eventlog.Warning | eventlog.Error)
		if err := eventlog.InstallAsEventCreate(serviceName, eventTypes); err != nil &&
			!strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf(
				"event source %q is not registered and registration requires administrator rights; "+
					"register it once via the Alloy installer or by running Alloy as administrator (underlying error: %w)",
				serviceName, err)
		}
	}
	el, err := eventlog.Open(serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open event log: %w", err)
	}
	return &realEventLog{el: el}, nil
}
