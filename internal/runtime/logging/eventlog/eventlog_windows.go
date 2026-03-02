//go:build windows

package eventlog

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

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

// openEventLogWindows installs the event source (if needed) and opens the event log.
// It is the default EventLogOpener on Windows.
func openEventLogWindows(serviceName string) (EventLog, error) {
	eventTypes := uint32(eventlog.Info | eventlog.Warning | eventlog.Error)
	err := eventlog.InstallAsEventCreate(serviceName, eventTypes)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, fmt.Errorf("failed to install event source: %w", err)
	}
	el, err := eventlog.Open(serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open event log: %w", err)
	}
	return &realEventLog{el: el}, nil
}
