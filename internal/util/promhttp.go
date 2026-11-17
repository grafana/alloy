package util

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PromHTTPHandlerFor returns a metrics handler for g that reports its own
// failures to logger at debug level. Pass any other options through opts. An
// ErrorLog already set on opts wins, so a caller that wants a different
// destination or level can supply one. A nil logger disables the reporting.
//
// The debug level is a deliberate trade-off between log noise and the ability
// to troubleshoot when needed.
//
// Without ErrorLog, promhttp drops the reason a metrics handler failed. It puts
// the reason in the HTTP response body, but the Prometheus scrape client
// discards that body and reports only "server returned HTTP status 500 Internal
// Server Error". The user then has no way to find the cause. This handler puts
// that reason in Alloy's debug logs instead.
func PromHTTPHandlerFor(g prometheus.Gatherer, logger *slog.Logger, opts promhttp.HandlerOpts) http.Handler {
	if opts.ErrorLog == nil {
		opts.ErrorLog = promHTTPErrorLogger(logger)
	}
	return promhttp.HandlerFor(g, opts)
}

// promHTTPErrorLogger adapts logger to the promhttp.Logger interface. It
// returns nil for a nil logger, which leaves promhttp reporting nothing.
func promHTTPErrorLogger(logger *slog.Logger) promhttp.Logger {
	if logger == nil {
		return nil
	}
	return &promhttpErrorLogger{logger: logger}
}

type promhttpErrorLogger struct {
	logger *slog.Logger
}

// Println implements promhttp.Logger.
func (l *promhttpErrorLogger) Println(v ...any) {
	// Sprintln puts a space between every operand, which Sprint does not do when
	// an operand is a string. Drop the newline Sprintln adds at the end.
	l.logger.Debug("metrics handler error", "err", strings.TrimSuffix(fmt.Sprintln(v...), "\n"))
}
