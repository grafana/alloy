package util

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// promhttpErrorLogger adapts an *slog.Logger to the promhttp.Logger interface.
type promhttpErrorLogger struct {
	logger *slog.Logger
}

// PromHTTPErrorLogger returns a promhttp.Logger that writes to logger at debug
// level. Pass the result as promhttp.HandlerOpts.ErrorLog.
//
// Without this, promhttp drops the reason a metrics handler failed. It puts the
// reason in the HTTP response body, but the Prometheus scrape client discards
// that body and reports only "server returned HTTP status 500 Internal Server
// Error". The user then has no way to find the cause. For example, a
// prometheus.exporter.snmp module that declares a table as a scalar makes the
// collector emit the same series more than once, and only the response body
// names the series.
//
// The level is debug on purpose. Users can enable debug logging to troubleshoot,
// while we can keep the noise minimal for normal operation.
//
// PromHTTPErrorLogger returns nil if logger is nil. When nil is given to promhttp
// it defaults to reporting nothing.
func PromHTTPErrorLogger(logger *slog.Logger) promhttp.Logger {
	if logger == nil {
		return nil
	}
	return &promhttpErrorLogger{logger: logger}
}

// Println implements promhttp.Logger.
func (l *promhttpErrorLogger) Println(v ...any) {
	// Sprintln puts a space between every operand, which Sprint does not do when
	// an operand is a string. Drop the newline Sprintln adds at the end.
	l.logger.Debug("metrics handler error", "err", strings.TrimSuffix(fmt.Sprintln(v...), "\n"))
}
