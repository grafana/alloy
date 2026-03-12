package stages

import (
	"encoding/json"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type DockerConfig struct{}

// NewDocker creates a predefined pipeline for parsing entries in the Docker json log format.
func NewDocker(logger log.Logger, _ prometheus.Registerer, _ featuregate.Stability) (Stage, error) {
	return toStage(&DockerStage{logger}), nil
}

type DockerStage struct {
	logger log.Logger
}

// DockerLog represents the expected json format written by docker:
// https://docs.docker.com/engine/logging/drivers/json-file/
type DockerLog struct {
	Log    string `json:"log"`
	Time   string `json:"time"`
	Stream string `json:"stream"`
}

const (
	dockerStream    = "stream"
	dockerOutput    = "output"
	dockerTimestamp = "timestamp"
)

func (d *DockerStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	var parsed DockerLog
	if err := json.Unmarshal([]byte(*entry), &parsed); err != nil {
		if Debug {
			level.Debug(d.logger).Log("msg", "failed to parse docker log", "err", err)
		}
		return
	}

	// NOTE: json.Unmarshal will happily parse any JSON and produce a zero-value struct.
	// To protect against incorrect usage, validate that the log field is present.
	if parsed.Log == "" {
		if Debug {
			level.Debug(d.logger).Log("msg", "not valid docker format")
		}
		return
	}

	// NOTE: Previous implementation used a "sub-pipeline"
	// to parse docker logs where the json stage added these fields
	// as "extracted" values so the other stages could operate on them.
	// We don't need this anymore but it would be a breaking change to
	// no longer set these.
	extracted[dockerOutput] = parsed.Log
	extracted[dockerStream] = parsed.Stream
	extracted[dockerTimestamp] = parsed.Time

	*entry = parsed.Log
	labels["stream"] = model.LabelValue(parsed.Stream)

	ts, err := time.Parse(time.RFC3339Nano, parsed.Time)
	if err == nil {
		*t = ts
	}
}
