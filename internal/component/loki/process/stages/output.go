package stages

import (
	"errors"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
)

// Config Errors.
var (
	ErrEmptyOutputStageConfig = errors.New("output stage config cannot be empty")
	ErrOutputSourceRequired   = errors.New("output source value is required if output is specified")
)

// OutputConfig initializes a configuration stage which sets the log line to a
// value from the extracted map.
type OutputConfig struct {
	Source string `alloy:"source,attr"`
}

// newOutputStage creates a new outputStage
func newOutputStage(logger log.Logger, config OutputConfig) (Stage, error) {
	if config.Source == "" {
		return nil, ErrOutputSourceRequired
	}
	return toStage(&outputStage{
		config: config,
		logger: logger,
	}), nil
}

// outputStage will mutate the incoming entry and set it from extracted data
type outputStage struct {
	config OutputConfig
	logger log.Logger
}

// Process implements Stage
func (o *outputStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	if v, ok := extracted[o.config.Source]; ok {
		s, err := getString(v)
		if err != nil {
			level.Debug(o.logger).Log("msg", "extracted output could not be converted to a string", "err", err, "type", reflect.TypeOf(v))
			return
		}
		*entry = s
	} else {
		level.Debug(o.logger).Log("msg", "extracted data did not contain output source")
	}
}

// Name implements Stage
func (o *outputStage) Name() string {
	return StageTypeOutput
}
