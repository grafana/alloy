package stages

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
)

var errOutputSourceRequired = errors.New("output source value is required if output is specified")

// OutputConfig initializes a configuration stage which sets the log line to a
// value from the extracted map.
type OutputConfig struct {
	Source string `alloy:"source,attr"`
}

func validateOutputConfig(config OutputConfig) error {
	if config.Source == "" {
		return errOutputSourceRequired
	}

	return nil
}

var (
	_ Stage          = (*outputStage)(nil)
	_ entryProcessor = (*outputStage)(nil)
)

// newOutputStage creates a new outputStage
func newOutputStage(logger *slog.Logger, config OutputConfig, next NextFn) (Stage, error) {
	if err := validateOutputConfig(config); err != nil {
		return nil, err
	}

	return &outputStage{
		next:   next,
		config: config,
		logger: logger.With("stage", "output"),
	}, nil
}

// outputStage will mutate the incoming entry and set it from extracted data
type outputStage struct {
	next   NextFn
	config OutputConfig
	logger *slog.Logger
}

// Run implements Stage.
func (o *outputStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return o.processEntry(e)
	})
}

// process implements stage.
func (o *outputStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = o.processEntry(entries[i])
	}
	return o.next(ctx, entries)
}

// Cleanup implements Stage.
func (o *outputStage) Cleanup() {}

func (o *outputStage) processEntry(e Entry) Entry {
	if v, ok := e.Extracted[o.config.Source]; ok {
		s, err := getString(v)
		if err != nil {
			o.logger.Debug("extracted output could not be converted to a string", "err", err, "type", reflect.TypeOf(v))
			return e
		}
		e.Line = s
	} else {
		o.logger.Debug("extracted data did not contain output source", "source", o.config.Source)
	}
	return e
}
