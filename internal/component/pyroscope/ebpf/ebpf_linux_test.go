//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestUnmarshalConfig(t *testing.T) {
	for _, tt := range []struct {
		name        string
		in          string
		expected    func() Arguments
		expectedErr string
	}{
		{
			name: "required-params-only",
			in: `
targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
`,
			expected: func() Arguments {
				x := NewDefaultArguments()
				x.Targets = []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"container_id": "cid",
						"service_name": "foo",
					}),
				}
				x.ForwardTo = []pyroscope.Appendable{}
				return x
			},
		},
		{
			name: "full-config",
			in: `
targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
collect_interval = "3s"
sample_rate = 239
`,
			expected: func() Arguments {
				x := NewDefaultArguments()
				x.Targets = []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"container_id": "cid",
						"service_name": "foo",
					}),
				}
				x.ForwardTo = []pyroscope.Appendable{}
				x.CollectInterval = time.Second * 3
				x.SampleRate = 239
				return x
			},
		},
		{
			name: "with-off-cpu-threshold",
			in: `
	targets = [{"service_name" = "foo", "container_id"= "cid"}]
	forward_to = []
	off_cpu_threshold = 1
	`,
			expected: func() Arguments {
				x := NewDefaultArguments()
				x.Targets = []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"container_id": "cid",
						"service_name": "foo",
					}),
				}
				x.ForwardTo = []pyroscope.Appendable{}
				x.OffCPUThreshold = 1
				return x
			},
		},
		{
			name: "syntax-problem",
			in: `
targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
collect_interval = 3s"
`,
			expectedErr: "4:21: expected TERMINATOR, got IDENT (and 1 more diagnostics)",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			arg := Arguments{}
			if tt.expectedErr != "" {
				err := syntax.Unmarshal([]byte(tt.in), &arg)
				require.Error(t, err)
				require.Equal(t, tt.expectedErr, err.Error())
				return
			}
			require.NoError(t, syntax.Unmarshal([]byte(tt.in), &arg))
			require.Equal(t, tt.expected(), arg)
		})
	}
}

func TestReconstructionAfterError(t *testing.T) {
	// The goal here is to produce an error when trying to create the symbol cache directory.
	// To keep things simple, we create a file and attempt to use it as a directory.
	f, err := os.CreateTemp(t.TempDir(), "")
	defer os.RemoveAll(f.Name())
	require.NoError(t, err)
	invalidCachePath := filepath.Join(f.Name(), "symb.cache")

	logger := util.TestLogger(t)
	reg := prometheus.NewRegistry()

	args := NewDefaultArguments()
	args.SymbCachePath = invalidCachePath
	_, err = New(logger, reg, "test-ebpf", args)
	require.Error(t, err)

	args = NewDefaultArguments()
	_, err = New(logger, reg, "test-ebpf", args)
	require.NoError(t, err)
}
