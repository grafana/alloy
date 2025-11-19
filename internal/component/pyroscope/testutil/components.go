//go:build linux && (arm64 || amd64)

package testutil

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"
)

// CreateWriteComponent creates a pyroscope.write component that forwards to the given endpoint
func CreateWriteComponent(l log.Logger, reg prometheus.Registerer, endpoint string) (pyroscope.Appendable, error) {
	var receiver pyroscope.Appendable
	e := write.GetDefaultEndpointOptions()
	e.URL = endpoint

	_, err := write.New(
		log.With(l, "component", "pyroscope.write"),
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
		return nil, fmt.Errorf("error creating write component: %w", err)
	}
	return receiver, nil
}
