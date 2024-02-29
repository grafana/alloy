package server

import (
	"runtime"
	"strings"

	"github.com/go-kit/log/level"

	"github.com/go-kit/log"
	el "golang.org/x/sys/windows/svc/eventlog"
)

// Default name for the Grafana Agent under Windows
const ServiceName = "Grafana Agent"

// NewWindowsEventLogger creates a new logger that writes to the event log
func NewWindowsEventLogger(cfg *Config) *Logger {
	return newLogger(cfg, makeWindowsEventLogger)
}

func makeWindowsEventLogger(cfg *Config) (log.Logger, error) {
	// Set up the log in windows events
	err := el.InstallAsEventCreate(ServiceName, el.Error|el.Info|el.Warning)

	// Agent should expect an error of 'already exists' if the Event Log sink has already previously been installed
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}
	il, err := el.Open(ServiceName)
	if err != nil {
		return nil, err
	}

	// Ensure the logger gets closed when the GC runs. It's valid to have more than one win logger open concurrently.
	runtime.SetFinalizer(il, func(l *el.Log) {
		l.Close()
	})

	// These are set up to be writers for each Windows log level
	// Set up this way so we can utilize all the benefits of logformatter
	infoLogger := newWinLogWrapper(cfg.LogFormat, func(p []byte) error {
		return il.Info(1, string(p))
	})
	warningLogger := newWinLogWrapper(cfg.LogFormat, func(p []byte) error {
		return il.Warning(1, string(p))
	})

	errorLogger := newWinLogWrapper(cfg.LogFormat, func(p []byte) error {
		return il.Error(1, string(p))
	})

	wl := &winLogger{
		errorLogger:   errorLogger,
		infoLogger:    infoLogger,
		warningLogger: warningLogger,
	}
	return level.NewFilter(wl, cfg.LogLevel.Level.Option), nil
}

// Looks through the key value pairs in the log for level and extract the value
func getLevel(keyvals ...interface{}) level.Value {
	for i := 0; i < len(keyvals); i++ {
		if vo, ok := keyvals[i].(level.Value); ok {
			return vo
		}
	}
	return nil
}

func newWinLogWrapper(format string, write func(p []byte) error) log.Logger {
	infoWriter := &winLogWriter{writer: write}
	infoLogger := log.NewLogfmtLogger(infoWriter)
	if format == "json" {
		infoLogger = log.NewJSONLogger(infoWriter)
	}
	return infoLogger
}

type winLogger struct {
	errorLogger   log.Logger
	infoLogger    log.Logger
	warningLogger log.Logger
}

func (w *winLogger) Log(keyvals ...interface{}) error {
	lvl := getLevel(keyvals...)
	// 3 different loggers are used so that agent can utilize the formatting features of go-kit logging
	// if agent did not use this then the windows logger uses different function calls for different levels
	// this is paired with the fact that the io.Writer interface only gives a byte array.
	switch lvl {
	case level.DebugValue():
		return w.infoLogger.Log(keyvals...)
	case level.InfoValue():
		return w.infoLogger.Log(keyvals...)
	case level.WarnValue():
		return w.warningLogger.Log(keyvals...)
	case level.ErrorValue():
		return w.errorLogger.Log(keyvals...)
	default:
		return w.infoLogger.Log(keyvals...)
	}
}

type winLogWriter struct {
	writer func(p []byte) error
}

func (i *winLogWriter) Write(p []byte) (n int, err error) {
	return len(p), i.writer(p)
}
