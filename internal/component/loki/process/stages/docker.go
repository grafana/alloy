package stages

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/featuregate"
)

type DockerConfig struct{}

var (
	_ Stage = (*dockerStage)(nil)
	_ stage = (*dockerStage)(nil)
)

func newDockerStage(logger *slog.Logger, _ prometheus.Registerer, _ featuregate.Stability, next NextFn) *dockerStage {
	return &dockerStage{next: next, logger: logger.With("stage", "docker")}
}

type dockerStage struct {
	next   NextFn
	logger *slog.Logger
}

// dockerLog represents the expected json format written by docker:
// https://docs.docker.com/engine/logging/drivers/json-file/
type dockerLog struct {
	Log    string `json:"log"`
	Time   string `json:"time"`
	Stream string `json:"stream"`
}

const (
	dockerStream    = "stream"
	dockerOutput    = "output"
	dockerTimestamp = "timestamp"
)

func (d *dockerStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return d.processEntry(e)
	})
}

// process implements stage and is only used by our new pipeline.
func (d *dockerStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = d.processEntry(entries[i])
	}

	return d.next(ctx, entries)
}

func (d *dockerStage) processEntry(e Entry) Entry {
	var parsed dockerLog
	if err := json.Unmarshal([]byte(e.Line), &parsed); err != nil {
		if debugEnabled(d.logger) {
			d.logger.Debug("failed to parse docker log", "err", err)
		}
		return e
	}

	// NOTE: json.Unmarshal will happily parse any JSON and produce a zero-value struct.
	// To protect against incorrect usage, validate that the log field is present.
	if parsed.Log == "" {
		if debugEnabled(d.logger) {
			d.logger.Debug("not valid docker format")
		}
		return e
	}

	// NOTE: Previous implementation used a "sub-pipeline"
	// to parse docker logs where the json stage added these fields
	// as "extracted" values so the other stages could operate on them.
	// We don't need this anymore but it would be a breaking change to
	// no longer set these.
	e.Extracted[dockerOutput] = parsed.Log
	e.Extracted[dockerStream] = parsed.Stream
	e.Extracted[dockerTimestamp] = parsed.Time

	e.Line = parsed.Log
	e.Labels[dockerStream] = model.LabelValue(parsed.Stream)

	ts, err := time.Parse(time.RFC3339Nano, parsed.Time)
	if err == nil {
		e.Timestamp = ts
	}
	return e
}

func (d *dockerStage) Cleanup() {}
