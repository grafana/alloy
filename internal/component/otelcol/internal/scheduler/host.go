package scheduler

import (
	"github.com/go-kit/log"

	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

// Host implements otelcomponent.Host for Grafana Alloy.
type Host struct {
	log log.Logger

	extensions map[otelcomponent.ID]otelcomponent.Component
	exporters  map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component
}

// NewHost creates a new Host.
func NewHost(l log.Logger, opts ...HostOption) *Host {
	h := &Host{log: l}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// HostOption customizes behavior of the Host.
type HostOption func(*Host)

// WithHostExtensions provides a custom set of extensions to the Host.
func WithHostExtensions(extensions map[otelcomponent.ID]otelcomponent.Component) HostOption {
	return func(h *Host) {
		h.extensions = extensions
	}
}

// WithHostExporters provides a custom set of exporters to the Host.
func WithHostExporters(exporters map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component) HostOption {
	return func(h *Host) {
		h.exporters = exporters
	}
}

var _ otelcomponent.Host = (*Host)(nil)

// GetExtensions implements otelcomponent.Host.
func (h *Host) GetExtensions() map[otelcomponent.ID]otelcomponent.Component {
	return h.extensions
}
