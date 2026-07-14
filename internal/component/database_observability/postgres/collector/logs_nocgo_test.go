//go:build !cgo

package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// Without cgo there is no SQL fingerprinting, so enable_error_logs_processing must fail
// collector creation loudly (the error reaches the component health status)
// instead of silently emitting nothing.
func TestNewLogs_ErrorLogsRequireCgo(t *testing.T) {
	_, err := NewLogs(LogsArguments{
		Receiver:        loki.NewLogsReceiver(),
		EntryHandler:    loki.NewEntryHandler(make(chan loki.Entry, 1), func() {}),
		Logger:          logging.NewSlogNop(),
		Registry:        prometheus.NewRegistry(),
		EnableErrorLogs: true,
	})
	require.ErrorContains(t, err, "enable_error_logs_processing requires a cgo-enabled Alloy build")
}
