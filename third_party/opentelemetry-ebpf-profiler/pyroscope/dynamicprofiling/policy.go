package dynamicprofiling // import "go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"

import (
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/discovery"
)

type Policy interface {
	ProfilingEnabled(process process.Process, containerID string) bool
}

type AlwaysOnPolicy struct{}

func (a AlwaysOnPolicy) ProfilingEnabled(_ process.Process, _ string) bool {
	return true
}

type ServiceDiscoveryTargetsOnlyPolicy struct {
	Discovery discovery.TargetProducer
}

func (s *ServiceDiscoveryTargetsOnlyPolicy) ProfilingEnabled(
	p process.Process,
	containerID string,
) bool {
	target := s.Discovery.FindTarget(uint32(p.PID()), containerID)
	return target != nil
}
