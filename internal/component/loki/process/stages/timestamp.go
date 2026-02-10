package stages

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"
	_ "time/tzdata" // embed timezone data

	"github.com/go-kit/log"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Config errors.
var (
	ErrEmptyTimestampStageConfig = errors.New("timestamp stage config cannot be empty")
	ErrTimestampSourceRequired   = errors.New("timestamp source value is required if timestamp is specified")
	ErrTimestampFormatRequired   = errors.New("timestamp format is required")
	ErrInvalidLocation           = errors.New("invalid location specified: %v")
	ErrInvalidActionOnFailure    = errors.New("invalid action on failure (supported values are %v)")
	ErrTimestampSourceMissing    = errors.New("extracted data did not contain a timestamp")
	ErrTimestampConversionFailed = errors.New("failed to convert extracted time to string")
	ErrTimestampParsingFailed    = errors.New("failed to parse time")

	Unix   = "Unix"
	UnixMs = "UnixMs"
	UnixUs = "UnixUs"
	UnixNs = "UnixNs"

	TimestampActionOnFailureSkip    = "skip"
	TimestampActionOnFailureFudge   = "fudge"
	TimestampActionOnFailureDefault = TimestampActionOnFailureFudge

	// Maximum number of "streams" for which we keep the last known timestamp
	maxLastKnownTimestampsCacheSize = 10000
)

// TimestampActionOnFailureOptions defines the available options for the
// `action_on_failure` field.
var TimestampActionOnFailureOptions = []string{TimestampActionOnFailureSkip, TimestampActionOnFailureFudge}

// TimestampConfig configures a processing stage for timestamp extraction.
type TimestampConfig struct {
	Source          string   `alloy:"source,attr"`
	Format          string   `alloy:"format,attr"`
	FallbackFormats []string `alloy:"fallback_formats,attr,optional"`
	Location        *string  `alloy:"location,attr,optional"`
	ActionOnFailure string   `alloy:"action_on_failure,attr,optional"`
}

type parser func(string) (time.Time, error)

func validateTimestampConfig(cfg *TimestampConfig) (parser, error) {
	if cfg.Source == "" {
		return nil, ErrTimestampSourceRequired
	}
	if cfg.Format == "" {
		return nil, ErrTimestampFormatRequired
	}
	var loc *time.Location
	var err error
	if cfg.Location != nil {
		loc, err = time.LoadLocation(*cfg.Location)
		if err != nil {
			return nil, fmt.Errorf("%v: %w", ErrInvalidLocation, err)
		}
	}

	// Validate the action on failure and enforce the default
	if cfg.ActionOnFailure == "" {
		cfg.ActionOnFailure = TimestampActionOnFailureDefault
	} else {
		if !slices.Contains(TimestampActionOnFailureOptions, cfg.ActionOnFailure) {
			return nil, fmt.Errorf(ErrInvalidActionOnFailure.Error(), TimestampActionOnFailureOptions)
		}
	}

	if len(cfg.FallbackFormats) > 0 {
		multiConvertDateLayout := func(input string) (time.Time, error) {
			originalTime, originalErr := convertDateLayout(cfg.Format, loc)(input)
			if originalErr == nil {
				return originalTime, originalErr
			}
			for i := 0; i < len(cfg.FallbackFormats); i++ {
				if t, err := convertDateLayout(cfg.FallbackFormats[i], loc)(input); err == nil {
					return t, err
				}
			}
			return originalTime, originalErr
		}
		return multiConvertDateLayout, nil
	}

	return convertDateLayout(cfg.Format, loc), nil
}

// newTimestampStage creates a new timestamp extraction pipeline stage.
func newTimestampStage(logger log.Logger, config TimestampConfig) (Stage, error) {
	parser, err := validateTimestampConfig(&config)
	if err != nil {
		return nil, err
	}

	var lastKnownTimestamps *lru.Cache
	if config.ActionOnFailure == TimestampActionOnFailureFudge {
		lastKnownTimestamps, err = lru.New(maxLastKnownTimestampsCacheSize)
		if err != nil {
			return nil, err
		}
	}

	return toStage(&timestampStage{
		config:              &config,
		logger:              logger,
		parser:              parser,
		lastKnownTimestamps: lastKnownTimestamps,
	}), nil
}

type timestampStage struct {
	config *TimestampConfig
	logger log.Logger
	parser parser

	// Stores the last known timestamp for a given "stream id" (guessed, since at this stage
	// there's no reliable way to know it).
	lastKnownTimestamps *lru.Cache
}

// Name implements Stage.
func (ts *timestampStage) Name() string {
	return StageTypeTimestamp
}

// Process implements Stage.
func (ts *timestampStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	if ts.config == nil {
		return
	}

	parsedTs, err := ts.parseTimestampFromSource(extracted)
	if err != nil {
		ts.processActionOnFailure(labels, t)
		return
	}

	// Update the log entry timestamp with the parsed one
	*t = *parsedTs

	// The timestamp has been correctly parsed, so we should store it in the map
	// containing the last known timestamp used by the "fudge" action on failure.
	if ts.config.ActionOnFailure == TimestampActionOnFailureFudge {
		ts.lastKnownTimestamps.Add(labels.String(), *t)
	}
}

func (ts *timestampStage) parseTimestampFromSource(extracted map[string]any) (*time.Time, error) {
	// Ensure the extracted data contains the timestamp source.
	v, ok := extracted[ts.config.Source]
	if !ok {
		level.Debug(ts.logger).Log("msg", ErrTimestampSourceMissing)
		return nil, ErrTimestampSourceMissing
	}

	// Convert the timestamp source to string (if it's not a string yet).
	s, err := getString(v)
	if err != nil {
		level.Debug(ts.logger).Log("msg", ErrTimestampConversionFailed, "err", err, "type", reflect.TypeOf(v))
		return nil, ErrTimestampConversionFailed
	}

	// Parse the timestamp source according to the configured format
	parsedTs, err := ts.parser(s)
	if err != nil {
		level.Debug(ts.logger).Log("msg", ErrTimestampParsingFailed, "err", err, "format", ts.config.Format, "value", s)

		return nil, ErrTimestampParsingFailed
	}

	return &parsedTs, nil
}

func (ts *timestampStage) processActionOnFailure(labels model.LabelSet, t *time.Time) {
	switch ts.config.ActionOnFailure {
	case TimestampActionOnFailureFudge:
		ts.processActionOnFailureFudge(labels, t)
	case TimestampActionOnFailureSkip:
		// Nothing to do
	}
}

func (ts *timestampStage) processActionOnFailureFudge(labels model.LabelSet, t *time.Time) {
	labelsStr := labels.String()
	lastTimestamp, ok := ts.lastKnownTimestamps.Get(labelsStr)

	// If the last known timestamp is unknown (i.e. has not been successfully parsed yet)
	// there's nothing we can do, so we're going to keep the current timestamp
	if !ok {
		return
	}

	// Fudge the timestamp
	*t = lastTimestamp.(time.Time).Add(1 * time.Nanosecond)

	// Store the fudged timestamp, so that a subsequent fudged timestamp will be 1ns after it
	ts.lastKnownTimestamps.Add(labelsStr, *t)
}
