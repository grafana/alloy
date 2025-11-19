// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interpreter // import "go.opentelemetry.io/ebpf-profiler/interpreter"

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/metrics"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
	"go.opentelemetry.io/ebpf-profiler/reporter"
	"go.opentelemetry.io/ebpf-profiler/tpbase"
)

// MultiData implements the Data interface for multiple interpreters.
type MultiData struct {
	interpreters []Data
}

// NewMultiData creates a new MultiData instance from multiple Data instances.
func NewMultiData(interpreters []Data) *MultiData {
	return &MultiData{
		interpreters: interpreters,
	}
}

// Attach attaches all interpreters and returns a MultiInstance.
func (m *MultiData) Attach(ebpf EbpfHandler, pid libpf.PID, bias libpf.Address,
	rm remotememory.RemoteMemory) (Instance, error) {
	var instances []Instance
	var errs []error

	for _, data := range m.interpreters {
		instance, err := data.Attach(ebpf, pid, bias, rm)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if instance != nil {
			instances = append(instances, instance)
		}
	}

	err := errors.Join(errs...)
	if len(instances) == 0 {
		// Either all interpreters returned nil instances without error (e.g., not ready yet)
		// in which case return nil, nil (valid state) otherwise return combined error.
		return nil, err
	}

	// We got at least one valid instance, log any errors that occurred
	if err != nil {
		log.Errorf("Errors occurred while attaching interpreters: %v", err)
	}

	return NewMultiInstance(instances), nil
}

// Unload unloads all interpreters.
func (m *MultiData) Unload(ebpf EbpfHandler) {
	for _, data := range m.interpreters {
		data.Unload(ebpf)
	}
}

// MultiInstance implements the Instance interface for multiple interpreters.
type MultiInstance struct {
	instances []Instance
}

// NewMultiInstance creates a new MultiInstance from multiple Instance instances.
func NewMultiInstance(instances []Instance) *MultiInstance {
	return &MultiInstance{
		instances: instances,
	}
}

// Detach detaches all interpreter instances.
func (m *MultiInstance) Detach(ebpf EbpfHandler, pid libpf.PID) error {
	var errs []error
	for _, instance := range m.instances {
		if err := instance.Detach(ebpf, pid); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SynchronizeMappings synchronizes mappings for all interpreter instances.
func (m *MultiInstance) SynchronizeMappings(ebpf EbpfHandler,
	exeReporter reporter.ExecutableReporter, pr process.Process, mappings []process.Mapping) error {
	var errs []error
	for _, instance := range m.instances {
		if err := instance.SynchronizeMappings(ebpf, exeReporter, pr, mappings); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// UpdateTSDInfo updates TSD info for all interpreter instances.
func (m *MultiInstance) UpdateTSDInfo(ebpf EbpfHandler, pid libpf.PID, info tpbase.TSDInfo) error {
	var errs []error
	for _, instance := range m.instances {
		if err := instance.UpdateTSDInfo(ebpf, pid, info); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Symbolize tries to symbolize the frame with each interpreter instance until one succeeds.
func (m *MultiInstance) Symbolize(ebpfFrame *host.Frame, frames *libpf.Frames) error {
	// Try each interpreter in order
	for _, instance := range m.instances {
		err := instance.Symbolize(ebpfFrame, frames)
		if err != ErrMismatchInterpreterType {
			return err
		}
	}
	return ErrMismatchInterpreterType
}

// GetAndResetMetrics collects metrics from all interpreter instances.
func (m *MultiInstance) GetAndResetMetrics() ([]metrics.Metric, error) {
	var allMetrics []metrics.Metric
	var errs []error

	for _, instance := range m.instances {
		metrics, err := instance.GetAndResetMetrics()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		allMetrics = append(allMetrics, metrics...)
	}

	return allMetrics, errors.Join(errs...)
}

func (m *MultiInstance) ReleaseResources() error {
	var errs []error
	for _, instance := range m.instances {
		err := instance.ReleaseResources()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
