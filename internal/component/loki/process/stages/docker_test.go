package stages

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/loki/pkg/push"
)

var (
	dockerRaw = `{"log":"level=info ts=2019-04-30T02:12:41.844179Z caller=filetargetmanager.go:180 msg=\"Adding target\" key=\"{com_docker_deploy_namespace=\\\"docker\\\", com_docker_fry=\\\"compose.api\\\", com_docker_image_tag=\\\"v0.4.12\\\", container_name=\\\"compose\\\", instance=\\\"compose-api-cbff6dfc9-cqfr8\\\", job=\\\"docker/compose-api\\\", namespace=\\\"docker\\\", pod_template_hash=\\\"769928975\\\"}\"\n","stream":"stderr","time":"2019-04-30T02:12:41.8443515Z"}`

	dockerProcessed = `level=info ts=2019-04-30T02:12:41.844179Z caller=filetargetmanager.go:180 msg="Adding target" key="{com_docker_deploy_namespace=\"docker\", com_docker_fry=\"compose.api\", com_docker_image_tag=\"v0.4.12\", container_name=\"compose\", instance=\"compose-api-cbff6dfc9-cqfr8\", job=\"docker/compose-api\", namespace=\"docker\", pod_template_hash=\"769928975\"}"
`
	dockerInvalidTimestampRaw = `{"log":"log message\n","stream":"stderr","time":"hi!"}`
	dockerTestTimeNow         = time.Now()
)

func TestDocker(t *testing.T) {
	type testCase struct {
		name     string
		input    loki.Entry
		expected loki.Entry
	}

	tests := []testCase{
		{
			name: "happy path",
			input: loki.Entry{
				Entry: push.Entry{
					Line:      dockerRaw,
					Timestamp: time.Now(),
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{"stream": "stderr"},
				Entry: push.Entry{
					Line:      dockerProcessed,
					Timestamp: time.Date(2019, 4, 30, 02, 12, 41, 844351500, time.UTC),
				},
			},
		},
		{
			name: "invalid timestamp",
			input: loki.Entry{
				Entry: push.Entry{
					Line:      dockerInvalidTimestampRaw,
					Timestamp: dockerTestTimeNow,
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{"stream": "stderr"},
				Entry: push.Entry{
					Line:      "log message\n",
					Timestamp: dockerTestTimeNow,
				},
			},
		},
		{
			name: "not json",
			input: loki.Entry{
				Entry: push.Entry{
					Line:      "i'm not json!",
					Timestamp: dockerTestTimeNow,
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{},
				Entry: push.Entry{
					Line:      "i'm not json!",
					Timestamp: dockerTestTimeNow,
				},
			},
		},
		{
			name: "json but not docker format",
			input: loki.Entry{
				Entry: push.Entry{
					Line:      `{"msg": "test"}`,
					Timestamp: dockerTestTimeNow,
				},
			},
			expected: loki.Entry{
				Labels: model.LabelSet{},
				Entry: push.Entry{
					Line:      `{"msg": "test"}`,
					Timestamp: dockerTestTimeNow,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := NewDocker(log.NewNopLogger(), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				t.Fatalf("failed to create Docker parser: %s", err)
			}
			out := processEntries(p, newEntry(nil, tt.input.Labels, tt.input.Line, tt.input.Timestamp))[0]

			require.EqualValues(t, tt.expected.Labels, out.Entry.Labels)
			require.Equal(t, tt.expected.Entry.Line, out.Entry.Line)
			require.Equal(t, tt.expected.Entry.Timestamp, out.Entry.Timestamp)
		})
	}
}

var (
	benchDockerTime  = time.Now()
	benchDockerEntry Entry
	benchDockerLine  = `{"log": "my cool logline", "stream": "stdout", "time": "2019-01-01T01:00:00.000000001Z"}`
)

func BenchmarkDocker(b *testing.B) {
	p, _ := NewDocker(log.NewNopLogger(), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	e := newEntry(nil, model.LabelSet{}, benchDockerLine, benchDockerTime)
	in := make(chan Entry)
	out := p.Run(in)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		in <- e
		benchDockerEntry = <-out
	}
}
