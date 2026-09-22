package util

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

// duplicateCollector emits the same series three times with no label to tell
// the rows apart. This is what prometheus.exporter.snmp does when an snmp.yml
// module declares a table as a scalar.
type duplicateCollector struct{}

func (duplicateCollector) Describe(chan<- *prometheus.Desc) {}

func (duplicateCollector) Collect(ch chan<- prometheus.Metric) {
	desc := prometheus.NewDesc("eip_cpu_load", "help", nil, nil)
	for i := range 3 {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(i))
	}
}

func serve(t *testing.T, c prometheus.Collector, logger *slog.Logger, opts promhttp.HandlerOpts) *httptest.ResponseRecorder {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	rec := httptest.NewRecorder()
	PromHTTPHandlerFor(reg, logger, opts).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec
}

func debugLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// Drop the timestamp so the assertions can compare the whole line.
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}
	return slog.New(slog.NewTextHandler(&buf, opts)), &buf
}

// TestPromHTTPHandlerFor_LogFormat pins down exactly what an operator sees.
func TestPromHTTPHandlerFor_LogFormat(t *testing.T) {
	logger, logs := debugLogger()

	rec := serve(t, duplicateCollector{}, logger, promhttp.HandlerOpts{})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t,
		`level=DEBUG msg="metrics handler error" err="error gathering metrics: `+
			`2 error(s) occurred:\n`+
			`* collected metric \"eip_cpu_load\" { gauge:{value:1}} was collected before with the same name and label values\n`+
			`* collected metric \"eip_cpu_load\" { gauge:{value:2}} was collected before with the same name and label values"`+"\n",
		logs.String())
}

// TestPromHTTPHandlerFor_ContinueOnError covers the CollectorIntegration case.
// The handler answers 200 with partial data, so the log is the only signal.
func TestPromHTTPHandlerFor_ContinueOnError(t *testing.T) {
	logger, logs := debugLogger()

	rec := serve(t, duplicateCollector{}, logger, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "eip_cpu_load 0")
	require.Contains(t, logs.String(), "was collected before with the same name and label values")
}

// TestPromHTTPHandlerFor_QuietAboveDebug makes sure the reporting stays off
// until the user turns on debug logging.
func TestPromHTTPHandlerFor_QuietAboveDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rec := serve(t, duplicateCollector{}, logger, promhttp.HandlerOpts{})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Empty(t, buf.String())
}

// recordingLogger is a promhttp.Logger a caller may already have.
type recordingLogger struct{ lines []string }

func (l *recordingLogger) Println(v ...any) { l.lines = append(l.lines, fmt.Sprintln(v...)) }

// TestPromHTTPHandlerFor_KeepsCallerErrorLog makes sure an ErrorLog set by the
// caller survives. The helper must not replace it with the default logger.
func TestPromHTTPHandlerFor_KeepsCallerErrorLog(t *testing.T) {
	logger, logs := debugLogger()
	caller := &recordingLogger{}

	rec := serve(t, duplicateCollector{}, logger, promhttp.HandlerOpts{ErrorLog: caller})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Len(t, caller.lines, 1)
	require.Contains(t, caller.lines[0], "was collected before with the same name and label values")
	require.Empty(t, logs.String(), "the slog logger must stay unused")
}

func TestPromHTTPHandlerFor_NilLogger(t *testing.T) {
	rec := serve(t, duplicateCollector{}, nil, promhttp.HandlerOpts{})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestPromHTTPHandlerFor_Success confirms the helper stays out of the way when
// nothing fails.
func TestPromHTTPHandlerFor_Success(t *testing.T) {
	logger, logs := debugLogger()

	c := prometheus.NewGauge(prometheus.GaugeOpts{Name: "eip_cpu_load", Help: "help"})
	c.Set(42)
	rec := serve(t, c, logger, promhttp.HandlerOpts{})

	require.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.Contains(t, string(body), "eip_cpu_load 42")
	require.Empty(t, logs.String())
}
