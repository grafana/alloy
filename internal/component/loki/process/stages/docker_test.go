package stages

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/featuregate"
)

var (
	dockerRaw       = `{"log":"level=info ts=2019-04-30T02:12:41.844179Z caller=filetargetmanager.go:180 msg=\"Adding target\" key=\"{com_docker_deploy_namespace=\\\"docker\\\", com_docker_fry=\\\"compose.api\\\", com_docker_image_tag=\\\"v0.4.12\\\", container_name=\\\"compose\\\", instance=\\\"compose-api-cbff6dfc9-cqfr8\\\", job=\\\"docker/compose-api\\\", namespace=\\\"docker\\\", pod_template_hash=\\\"769928975\\\"}\"\n","stream":"stderr","time":"2019-04-30T02:12:41.8443515Z"}`
	dockerProcessed = `level=info ts=2019-04-30T02:12:41.844179Z caller=filetargetmanager.go:180 msg="Adding target" key="{com_docker_deploy_namespace=\"docker\", com_docker_fry=\"compose.api\", com_docker_image_tag=\"v0.4.12\", container_name=\"compose\", instance=\"compose-api-cbff6dfc9-cqfr8\", job=\"docker/compose-api\", namespace=\"docker\", pod_template_hash=\"769928975\"}"
`
	dockerInvalidTimestampRaw = `{"log":"log message\n","stream":"stderr","time":"hi!"}`
	dockerTestTimeNow         = time.Now()
)

func TestDocker(t *testing.T) {
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal("could not parse timezone", err)
	}

	tests := map[string]struct {
		entry          string
		expectedEntry  string
		t              time.Time
		expectedT      time.Time
		labels         map[string]string
		expectedLabels map[string]string
	}{
		"happy path": {
			dockerRaw,
			dockerProcessed,
			time.Now(),
			time.Date(2019, 4, 30, 02, 12, 41, 844351500, loc),
			map[string]string{},
			map[string]string{
				"stream": "stderr",
			},
		},
		"invalid timestamp": {
			dockerInvalidTimestampRaw,
			"log message\n",
			dockerTestTimeNow,
			dockerTestTimeNow,
			map[string]string{},
			map[string]string{
				"stream": "stderr",
			},
		},
		"invalid json": {
			"i'm not json!",
			"i'm not json!",
			dockerTestTimeNow,
			dockerTestTimeNow,
			map[string]string{},
			map[string]string{},
		},
	}

	for tName, tt := range tests {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()
			p, err := NewDocker(log.NewNopLogger(), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if err != nil {
				t.Fatalf("failed to create Docker parser: %s", err)
			}
			out := processEntries(p, newEntry(nil, toLabelSet(tt.labels), tt.entry, tt.t))[0]

			assertLabels(t, tt.expectedLabels, out.Labels)
			assert.Equal(t, tt.expectedEntry, out.Line, "did not receive expected log entry")
			if out.Timestamp.Unix() != tt.expectedT.Unix() {
				t.Fatalf("mismatch ts want: %s got:%s", tt.expectedT, tt.t)
			}
		})
	}
}
