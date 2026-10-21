//go:build linux && (arm64 || amd64)

package ebpf

import (
	"context"
	"fmt"

	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/ebpf-profiler/probes/kprobe"
	"go.opentelemetry.io/ebpf-profiler/probes/offcpu"
	"go.opentelemetry.io/ebpf-profiler/tracer"
)

type probeProvider interface {
	Probe() tracer.Probe
}

func (args *Arguments) probes(ctx context.Context) ([]tracer.Probe, error) {
	var probes []tracer.Probe
	if args.OffCPUThreshold > 0 {
		p, err := newProbe(ctx, offcpu.NewFactory(), &offcpu.Config{Threshold: args.OffCPUThreshold})
		if err != nil {
			return nil, fmt.Errorf("off-cpu probe: %w", err)
		}
		probes = append(probes, p)
	}
	for _, link := range args.probeLinks() {
		spec, err := tracer.ParseProbe(link)
		if err != nil {
			return nil, fmt.Errorf("probe link %q: %w", link, err)
		}
		p, err := newProbe(ctx, kprobe.NewFactory(), &kprobe.Config{
			Mode:   spec.Mode.String(),
			Symbol: spec.Symbol,
			Target: spec.Target,
		})
		if err != nil {
			return nil, fmt.Errorf("probe link %q: %w", link, err)
		}
		probes = append(probes, p)
	}
	return probes, nil
}

func newProbe(ctx context.Context, f extension.Factory, cfg interface{ Validate() error }) (tracer.Probe, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	ext, err := f.Create(ctx, extension.Settings{ID: otelcomponent.NewID(f.Type())}, cfg)
	if err != nil {
		return nil, err
	}
	pp, ok := ext.(probeProvider)
	if !ok {
		return nil, fmt.Errorf("extension %s does not provide a probe", f.Type())
	}
	return pp.Probe(), nil
}
