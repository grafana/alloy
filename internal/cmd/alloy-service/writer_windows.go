package main

import (
	"io"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

// writer sends writes to the Windows Event Log.
type writer struct {
	el *eventlog.Log
}

var (
	_ io.Writer = (*writer)(nil)
)

// newWriter creates a new writer which writes to the Windows Event
// Logger.
func newWriter() (*writer, error) {
	// Fast path: if the event source is already registered, open it directly.
	// eventlog.InstallAsEventCreate opens the parent EventLog\Application key
	// with CREATE_SUB_KEY access, which requires admin. When Alloy runs under a
	// dedicated non-admin service account, that call fails with "Access is
	// denied" even though the source already exists, causing the process to
	// exit before svc.Run can report back to the SCM (surfacing as service
	// error 1053).
	if el, err := eventlog.Open(serviceName); err == nil {
		runtime.SetFinalizer(el, func(li *eventlog.Log) {
			_ = li.Close()
		})
		return &writer{el: el}, nil
	}

	eventTypes := uint32(eventlog.Info | eventlog.Warning | eventlog.Error)

	// Install the event source. This will fail with an error string saying "already
	// exists" if it has been installed before.
	if err := eventlog.InstallAsEventCreate(serviceName, eventTypes); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		return nil, err
	}

	el, err := eventlog.Open(serviceName)
	if err != nil {
		return nil, err
	}

	// Ensure the logger gets closed when GC runs.
	runtime.SetFinalizer(el, func(li *eventlog.Log) {
		_ = li.Close()
	})

	return &writer{el: el}, nil
}

var (
	warnText  = "warn"
	errorText = "error"
)

// Write implements [io.Writer], writing the provided data to the event logger.
// If the data contains the phrase "warn," then the text is logged as a
// warn-level event. If the data contains the phrase "error," then the text is
// logged as an error-level event.
func (l *writer) Write(data []byte) (n int, err error) {
	var (
		leveledLogger = l.el.Info
		msg           = string(data)
	)

	// TODO(rfratto): Find a way to reduce the amount of false positives where
	// log lines get incorrectly flagged as warning/error log lines.
	//
	// A longer-term solution would need to consider that logs may be emitted as
	// either logfmt or JSON.
	switch {
	case strings.Contains(msg, warnText):
		leveledLogger = l.el.Warning
	case strings.Contains(msg, errorText):
		leveledLogger = l.el.Error
	}

	if err := leveledLogger(1, msg); err != nil {
		return 0, err
	}
	return len(data), nil
}
