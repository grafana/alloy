//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
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
	off_cpu_threshold = 1000
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
				x.OffCPUThreshold = 1000
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
