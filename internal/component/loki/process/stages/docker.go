package stages

import (
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	RFC3339Nano = "RFC3339Nano"
)

// DockerConfig is an empty struct that is used to enable a pre-defined
// pipeline for decoding entries that are using the Docker logs format.
type DockerConfig struct{}

// NewDocker creates a predefined pipeline for parsing entries in the Docker
// json log format.
func NewDocker(logger log.Logger, registerer prometheus.Registerer, minStability featuregate.Stability) (Stage, error) {
	stages := []StageConfig{
		{
			JSONConfig: &JSONConfig{
				Expressions: map[string]string{
					"output":    "log",
					"stream":    "stream",
					"timestamp": "time",
				},
			},
		},
		{
			LabelsConfig: &LabelsConfig{
				Values: map[string]*string{"stream": nil},
			},
		},
		{
			TimestampConfig: &TimestampConfig{
				Source: "timestamp",
				Format: RFC3339Nano,
			},
		},
		{
			OutputConfig: &OutputConfig{
				"output",
			},
		},
	}
	return NewPipeline(logger, stages, nil, registerer, minStability)
}
