package main

import (
	"io"
	"strings"

	"github.com/grafana/alloy/internal/runtime/logging/eventlog"
)

// writer sends writes to the Windows Event Log.
type writer struct {
	el eventlog.EventLog
}

var (
	_ io.Writer = (*writer)(nil)
)

// newWriter creates a new writer which writes to the Windows Event Log.
//
// It uses the shared event log opener from internal/runtime/logging/eventlog,
// which pre-checks whether the source is already registered before attempting
// to install it. That lets a dedicated non-admin service account start the
// service when the Alloy installer (or a prior admin run) has already
// registered the "Alloy" event source, avoiding the CREATE_SUB_KEY access
// requirement of eventlog.InstallAsEventCreate that would otherwise cause the
// service to exit before svc.Run can report back to the SCM (surfacing as
// service error 1053).
func newWriter() (*writer, error) {
	el, err := eventlog.GetEventLogOpener()(serviceName)
	if err != nil {
		return nil, err
	}
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
