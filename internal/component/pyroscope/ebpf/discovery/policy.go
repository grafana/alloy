//go:build unix

package discovery

import "go.opentelemetry.io/ebpf-profiler/process"

type ServiceDiscoveryTargetsOnlyPolicy struct {
	Discovery TargetProducer
}

func (s *ServiceDiscoveryTargetsOnlyPolicy) ProfilingEnabled(
	p process.Process,
	containerID string,
) bool {

	target := s.Discovery.FindTarget(uint32(p.PID()), containerID)
	return target != nil
}
