package scheduler

import (
	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/flow/logging/level"

	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

// Host implements otelcomponent.Host for Grafana Agent Flow.
type Host struct {
	log log.Logger

	extensions map[otelcomponent.ID]otelextension.Extension
	exporters  map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component
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
func WithHostExtensions(extensions map[otelcomponent.ID]otelextension.Extension) HostOption {
	return func(h *Host) {
		h.extensions = extensions
	}
}

// WithHostExporters provides a custom set of exporters to the Host.
func WithHostExporters(exporters map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component) HostOption {
	return func(h *Host) {
		h.exporters = exporters
	}
}

var _ otelcomponent.Host = (*Host)(nil)

// ReportFatalError implements otelcomponent.Host.
func (h *Host) ReportFatalError(err error) {
	level.Error(h.log).Log("msg", "fatal error running component", "err", err)
}

// GetFactory implements otelcomponent.Host.
func (h *Host) GetFactory(kind otelcomponent.Kind, componentType otelcomponent.Type) otelcomponent.Factory {
	// GetFactory is used for components to create other components. It's not
	// clear if we want to allow this right now, so it's disabled.
	return nil
}

// GetExtensions implements otelcomponent.Host.
func (h *Host) GetExtensions() map[otelcomponent.ID]otelextension.Extension {
	return h.extensions
}

// GetExporters implements otelcomponent.Host.
func (h *Host) GetExporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return h.exporters
}
