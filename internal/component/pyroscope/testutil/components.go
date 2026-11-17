//go:build linux && (arm64 || amd64)

package testutil

import (
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
)

// CreateWriteComponent creates a pyroscope.write component that forwards to the given endpoint
func CreateWriteComponent(l *slog.Logger, reg prometheus.Registerer, endpoint string) (pyroscope.Appendable, *write.Component, error) {
	var receiver pyroscope.Appendable
	e := write.GetDefaultEndpointOptions()
	e.URL = endpoint

	c, err := write.New(
		l.With("component", "pyroscope.write"),
		noop.Tracer{},
		reg,
		func(exports write.Exports) {
			receiver = exports.Receiver
		},
		"test",
		"",
		write.Arguments{Endpoints: []*write.EndpointOptions{&e}},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating write component: %w", err)
	}
	return receiver, c, nil
}
