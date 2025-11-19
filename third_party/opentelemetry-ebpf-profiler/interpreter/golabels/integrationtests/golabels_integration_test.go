//go:build integration && linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtests

import (
	_ "embed"

	"context"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/tracer"
	tracertypes "go.opentelemetry.io/ebpf-profiler/tracer/types"
)

var (
	//go:embed pprof_1_23
	pprof_1_23 []byte

	//go:embed pprof_1_24
	pprof_1_24 []byte

	//go:embed pprof_1_24_cgo
	pprof_1_24_cgo []byte

	//go:embed pprof_1_24_cgo_pie
	pprof_1_24_cgo_pie []byte
)

type mockIntervals struct{}

func (mockIntervals) MonitorInterval() time.Duration    { return 1 * time.Second }
func (mockIntervals) TracePollInterval() time.Duration  { return 250 * time.Millisecond }
func (mockIntervals) PIDCleanupInterval() time.Duration { return 1 * time.Second }

func isRoot() bool {
	return os.Geteuid() == 0
}

func Test_Golabels(t *testing.T) {
	if !isRoot() {
		t.Skip("root privileges required")
	}

	tests := map[string]struct {
		bin []byte
	}{
		"pprof_1_23":         {bin: pprof_1_23},
		"pprof_1_24":         {bin: pprof_1_24},
		"pprof_1_24_cgo":     {bin: pprof_1_24_cgo},
		"pprof_1_24_cgo_pie": {bin: pprof_1_24_cgo_pie},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			exe, err := os.CreateTemp(t.TempDir(), name)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(exe.Name())

			if _, err = exe.Write(tc.bin); err != nil {
				t.Fatal(err)
			}
			if err = exe.Close(); err != nil {
				t.Fatal(err)
			}

			if err = os.Chmod(exe.Name(), 0o755); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			enabledTracers, _ := tracertypes.Parse("")
			enabledTracers.Enable(tracertypes.Labels)
			enabledTracers.Enable(tracertypes.GoTracer)

			trc, err := tracer.NewTracer(ctx, &tracer.Config{
				Intervals:              &mockIntervals{},
				IncludeTracers:         enabledTracers,
				SamplesPerSecond:       20,
				ProbabilisticInterval:  100,
				ProbabilisticThreshold: 100,
				OffCPUThreshold:        uint32(math.MaxUint32 / 100),
				VerboseMode:            true,
				Policy:                 dynamicprofiling.AlwaysOnPolicy{},
			})
			require.NoError(t, err)

			trc.StartPIDEventProcessor(ctx)

			err = trc.AttachTracer()
			require.NoError(t, err)

			t.Log("Attached tracer program")

			err = trc.EnableProfiling()
			require.NoError(t, err)

			err = trc.AttachSchedMonitor()
			require.NoError(t, err)

			traceCh := make(chan *host.Trace)

			err = trc.StartMapMonitors(ctx, traceCh)
			require.NoError(t, err)

			go func() {
				if err := exec.CommandContext(ctx, exe.Name()).Run(); err != nil {
					select {
					case <-ctx.Done():
						// Context is canceled, meaning the test is done.
					default:
						t.Log(err)
					}
				}
			}()

			for trace := range traceCh {
				if trace == nil {
					continue
				}
				if len(trace.CustomLabels) > 0 {
					hits := 0
					for k, v := range trace.CustomLabels {
						if strings.HasPrefix(k, "l1") {
							require.Len(t, v, 22)
							require.True(t, strings.HasPrefix(v, "label1"))
							hits |= (1 << 0)
						} else if strings.HasPrefix(k, "l2") {
							require.Len(t, v, 30)
							require.True(t, strings.HasPrefix(v, "label2"))
							hits |= (1 << 1)
						} else if strings.HasPrefix(k, "l3") {
							require.Len(t, v, 47)
							require.True(t, strings.HasPrefix(v, "label3"))
							hits |= (1 << 2)
						}
					}
					if hits == (1<<0 | 1<<1 | 1<<2) {
						cancel()
						break
					}
				}
			}
		})
	}
}
