//go:build linux && (arm64 || amd64)

package testutil

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/writer"
)

// CreateWriteComponent creates a pyroscope.write component that forwards to the given endpoint
func CreateWriteComponent(l *slog.Logger, reg prometheus.Registerer, endpoint string) (pyroscope.Appendable, *writer.Component, error) {
	var receiver pyroscope.Appendable
	e := writer.GetDefaultEndpointOptions()
	e.URL = endpoint

	dataPath := filepath.Join(os.TempDir(), "alloy-pyroscope-write-test")

	c, err := writer.New(
		l.With("component", "pyroscope.write"),
		noop.Tracer{},
		reg,
		func(exports writer.Exports) {
			receiver = exports.Receiver
		},
		"test",
		"",
		dataPath,
		writer.Arguments{Endpoints: []*writer.EndpointOptions{&e}},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating write component: %w", err)
	}
	return receiver, c, nil
}
