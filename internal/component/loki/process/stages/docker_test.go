package stages

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
)

var (
	dockerRaw = `{"log":"level=info ts=2019-04-30T02:12:41.844179Z caller=filetargetmanager.go:180 msg=\"Adding target\" key=\"{com_docker_deploy_namespace=\\\"docker\\\", com_docker_fry=\\\"compose.api\\\", com_docker_image_tag=\\\"v0.4.12\\\", container_name=\\\"compose\\\", instance=\\\"compose-api-cbff6dfc9-cqfr8\\\", job=\\\"docker/compose-api\\\", namespace=\\\"docker\\\", pod_template_hash=\\\"769928975\\\"}\"\n","stream":"stderr","time":"2019-04-30T02:12:41.8443515Z"}`

	dockerProcessed = `level=info ts=2019-04-30T02:12:41.844179Z caller=filetargetmanager.go:180 msg="Adding target" key="{com_docker_deploy_namespace=\"docker\", com_docker_fry=\"compose.api\", com_docker_image_tag=\"v0.4.12\", container_name=\"compose\", instance=\"compose-api-cbff6dfc9-cqfr8\", job=\"docker/compose-api\", namespace=\"docker\", pod_template_hash=\"769928975\"}"
`
	dockerProcessedTime       = time.Date(2019, 4, 30, 02, 12, 41, 844351500, time.UTC)
	dockerInvalidTimestampRaw = `{"log":"log message\n","stream":"stderr","time":"hi!"}`
	dockerTestTimeNow         = time.Now()
)

func TestDockerStage(t *testing.T) {
	type testCase struct {
		name     string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "happy path",
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, dockerRaw, dockerTestTimeNow),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"output": dockerProcessed, "stream": "stderr", "timestamp": "2019-04-30T02:12:41.8443515Z"},
					model.LabelSet{"stream": "stderr"},
					dockerProcessed,
					dockerProcessedTime,
				),
			},
		},
		{
			name: "invalid timestamp",
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, dockerInvalidTimestampRaw, dockerTestTimeNow),
			},
			expected: []Entry{
				newEntry(
					map[string]any{"output": "log message\n", "stream": "stderr", "timestamp": "hi!"},
					model.LabelSet{"stream": "stderr"},
					"log message\n",
					dockerTestTimeNow,
				),
			},
		},
		{
			name: "not json",
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm not json!", dockerTestTimeNow),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "i'm not json!", dockerTestTimeNow),
			},
		},
		{
			name: "json but not docker format",
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"msg": "test"}`, dockerTestTimeNow),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"msg": "test"}`, dockerTestTimeNow),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{DockerConfig: &DockerConfig{}}}, tt.entries, tt.expected, "")
		})
	}
}

func BenchmarkDockerStage(b *testing.B) {
	batch := loki.NewBatch()
	batch.Add(loki.NewStream(model.LabelSet{}, push.Entry{
		Timestamp: time.Now(),
		Line:      `{"log": "my cool logline", "stream": "stdout", "time": "2019-01-01T01:00:00.000000001Z"}`,
	}))
	runPipelineBenchmark(b, []StageConfig{{DockerConfig: &DockerConfig{}}}, batch)
}
