package main

import (
	"io"
	"runtime"
	"strings"

	"github.com/go-kit/log"
	"golang.org/x/sys/windows/svc/eventlog"
)

// logger sends logs to the Windows Event Log.
type logger struct {
	el *eventlog.Log
}

var (
	_ log.Logger = (*logger)(nil)
	_ io.Writer  = (*logger)(nil)
)

// newLogger creates a new logger which writes logs to the Windows Event
// Logger.
func newLogger() (*logger, error) {
	eventTypes := uint32(eventlog.Info | eventlog.Warning | eventlog.Error)

	// Install the event source. This will fail with an error string saying "already
	// exists" if it has been installed before.
	err := eventlog.InstallAsEventCreate(serviceName, eventTypes)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
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

	return &logger{el: el}, nil
}

// Log implements [log.Logger], logging the key-value pairs to the Windows
// event logger as logfmt.
//
// If kvps contains a logging level, then
func (l *logger) Log(kvps ...interface{}) error {
	// log.NewLogfmtLogger shouldn't escape to the heap since it's never used
	// beyond the scope of this initial call.
	return log.NewLogfmtLogger(l).Log(kvps...)
}

var (
	warnText  = "warn"
	errorText = "error"
)

// Write implements [io.Writer], writing the provided data to the event logger.
// If the data contains the phrase "warn," then the text is logged as a
// warn-level event. If the data contains the phrase "error," then the text is
// logged as an error-level event.
func (l *logger) Write(data []byte) (n int, err error) {
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
