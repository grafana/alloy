//go:build linux && (arm64 || amd64)

package ebpf

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/tracer"
	"go.uber.org/atomic"
)

type fakeProbe struct {
	unloads *atomic.Int32
}

func (p *fakeProbe) Load(context.Context, tracer.ProbeRegistrar, *tracer.ProbeContext) error {
	return nil
}

func (p *fakeProbe) Unload() error {
	p.unloads.Add(1)
	return nil
}

type fakeEnabler struct {
	failAt  int
	enabled int
}

func (e *fakeEnabler) EnableProbe(context.Context, tracer.Probe) error {
	if e.enabled == e.failAt {
		return errors.New("enable failed")
	}
	e.enabled++
	return nil
}

func newFakeProbes(n int, unloads *atomic.Int32) []tracer.Probe {
	probes := make([]tracer.Probe, 0, n)
	for range n {
		probes = append(probes, &fakeProbe{unloads: unloads})
	}
	return probes
}

func TestEnableProbes(t *testing.T) {
	var unloads atomic.Int32

	err := enableProbes(t.Context(), &fakeEnabler{failAt: -1}, newFakeProbes(2, &unloads), slog.Default())
	require.NoError(t, err)
	require.Zero(t, unloads.Load())
}

func TestEnableProbesUnloadsOnError(t *testing.T) {
	var unloads atomic.Int32

	err := enableProbes(t.Context(), &fakeEnabler{failAt: 1}, newFakeProbes(2, &unloads), slog.Default())
	require.Error(t, err)
	require.EqualValues(t, 2, unloads.Load())
}
