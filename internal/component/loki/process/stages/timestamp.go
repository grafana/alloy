package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"time"
	_ "time/tzdata" // embed timezone data

	lru "github.com/hashicorp/golang-lru"
)

var (
	errTimestampSourceRequired           = errors.New("timestamp source value is required if timestamp is specified")
	errTimestampFormatRequired           = errors.New("timestamp format is required")
	errInvalidLocation                   = errors.New("invalid location specified: %v")
	errInvalidActionOnFailure            = errors.New("invalid action on failure (supported values are %v)")
	errInvalidActionOnDuplicateTimestamp = errors.New("invalid action on duplicate timestamp (supported values are %v)")
	errTimestampSourceMissing            = errors.New("extracted data did not contain a timestamp")
	errTimestampConversionFailed         = errors.New("failed to convert extracted time to string")
	errTimestampParsingFailed            = errors.New("failed to parse time")

	timestampActionOnFailureSkip    = "skip"
	timestampActionOnFailureFudge   = "fudge"
	timestampActionOnFailureDefault = timestampActionOnFailureFudge

	timestampActionOnDuplicateTimestampKeep    = "keep"
	timestampActionOnDuplicateTimestampFudge   = "fudge"
	timestampActionOnDuplicateTimestampDefault = timestampActionOnDuplicateTimestampFudge

	// Maximum number of "streams" for which we keep the last known timestamp
	maxLastKnownTimestampsCacheSize = 10000
)

// timestampActionOnFailureOptions defines the available options for the
// `action_on_failure` field.
var timestampActionOnFailureOptions = []string{timestampActionOnFailureSkip, timestampActionOnFailureFudge}

// timestampActionOnDuplicateTimestampOptions defines the available options for the
// `action_on_duplicate_timestamp` field.
var timestampActionOnDuplicateTimestampOptions = []string{timestampActionOnDuplicateTimestampKeep, timestampActionOnDuplicateTimestampFudge}

// TimestampConfig configures a processing stage for timestamp extraction.
type TimestampConfig struct {
	Source                     string   `alloy:"source,attr"`
	Format                     string   `alloy:"format,attr"`
	FallbackFormats            []string `alloy:"fallback_formats,attr,optional"`
	Location                   *string  `alloy:"location,attr,optional"`
	ActionOnFailure            string   `alloy:"action_on_failure,attr,optional"`
	ActionOnDuplicateTimestamp string   `alloy:"action_on_duplicate_timestamp,attr,optional"`
}

type parser func(string) (time.Time, error)

func validateTimestampConfig(cfg *TimestampConfig) (parser, error) {
	if cfg.Source == "" {
		return nil, errTimestampSourceRequired
	}
	if cfg.Format == "" {
		return nil, errTimestampFormatRequired
	}
	var loc *time.Location
	var err error
	if cfg.Location != nil {
		loc, err = time.LoadLocation(*cfg.Location)
		if err != nil {
			return nil, fmt.Errorf("%v: %w", errInvalidLocation, err)
		}
	}

	// Validate the action on failure and enforce the default
	if cfg.ActionOnFailure == "" {
		cfg.ActionOnFailure = timestampActionOnFailureDefault
	} else {
		if !slices.Contains(timestampActionOnFailureOptions, cfg.ActionOnFailure) {
			return nil, fmt.Errorf(errInvalidActionOnFailure.Error(), timestampActionOnFailureOptions)
		}
	}

	// Validate the action on duplicate timestamp and enforce the default
	if cfg.ActionOnDuplicateTimestamp == "" {
		cfg.ActionOnDuplicateTimestamp = timestampActionOnDuplicateTimestampDefault
	} else {
		if !slices.Contains(timestampActionOnDuplicateTimestampOptions, cfg.ActionOnDuplicateTimestamp) {
			return nil, fmt.Errorf(errInvalidActionOnDuplicateTimestamp.Error(), timestampActionOnDuplicateTimestampOptions)
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

var (
	_ Stage          = (*timestampStage)(nil)
	_ entryProcessor = (*timestampStage)(nil)
)

// newTimestampStage creates a new timestamp extraction pipeline stage.
func newTimestampStage(logger *slog.Logger, config TimestampConfig, next NextFn) (*timestampStage, error) {
	parser, err := validateTimestampConfig(&config)
	if err != nil {
		return nil, err
	}

	var lastKnownTimestamps *lru.Cache
	if config.ActionOnFailure == timestampActionOnFailureFudge || config.ActionOnDuplicateTimestamp == timestampActionOnDuplicateTimestampFudge {
		lastKnownTimestamps, err = lru.New(maxLastKnownTimestampsCacheSize)
		if err != nil {
			return nil, err
		}
	}

	return &timestampStage{
		next:                next,
		config:              config,
		logger:              logger.With("stage", "timestamp"),
		parser:              parser,
		lastKnownTimestamps: lastKnownTimestamps,
	}, nil
}

// timestampCacheEntry holds both the original parsed timestamp and the last
// adjusted (output) timestamp for a given stream. lastParsed is used for
// equality comparison so fudging only triggers on truly duplicate inputs,
// while lastAdjusted is the base for computing the next +1ns offset.
type timestampCacheEntry struct {
	lastParsed   time.Time
	lastAdjusted time.Time
}

type timestampStage struct {
	next   NextFn
	config TimestampConfig
	logger *slog.Logger
	parser parser

	// Stores the last known timestamp for a given "stream id" (guessed, since at this stage
	// there's no reliable way to know it).
	lastKnownTimestamps *lru.Cache
}

// Run implements Stage.
func (ts *timestampStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return ts.processEntry(e)
	})
}

// process implements stage.
func (ts *timestampStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = ts.processEntry(entries[i])
	}
	return ts.next(ctx, entries)
}

// Cleanup implements Stage.
func (ts *timestampStage) Cleanup() {}

func (ts *timestampStage) processEntry(e Entry) Entry {
	parsedTs, err := ts.parseTimestampFromSource(e.Extracted)
	if err != nil {
		return ts.processActionOnFailure(e)
	}

	// Update the log entry timestamp with the parsed one
	e.Timestamp = *parsedTs

	// When action_on_duplicate_timestamp is fudge, ensure multiple messages with the
	// exact same parsed timestamp get distinct timestamps (lastKnown+1ns each) so
	// message order is preserved in Loki and Grafana.
	labelsStr := e.Labels.String()
	if ts.config.ActionOnDuplicateTimestamp == timestampActionOnDuplicateTimestampFudge && ts.lastKnownTimestamps != nil {
		if lastTimestamp, ok := ts.lastKnownTimestamps.Get(labelsStr); ok {
			entry := lastTimestamp.(timestampCacheEntry)
			if parsedTs.Equal(entry.lastParsed) {
				e.Timestamp = entry.lastAdjusted.Add(1 * time.Nanosecond)
			}
		}
	}
	if (ts.config.ActionOnFailure == timestampActionOnFailureFudge || ts.config.ActionOnDuplicateTimestamp == timestampActionOnDuplicateTimestampFudge) && ts.lastKnownTimestamps != nil {
		ts.lastKnownTimestamps.Add(labelsStr, timestampCacheEntry{lastParsed: *parsedTs, lastAdjusted: e.Timestamp})
	}

	return e
}

func (ts *timestampStage) parseTimestampFromSource(extracted map[string]any) (*time.Time, error) {
	// Ensure the extracted data contains the timestamp source.
	v, ok := extracted[ts.config.Source]
	if !ok {
		ts.logger.Debug(errTimestampSourceMissing.Error())
		return nil, errTimestampSourceMissing
	}

	// Convert the timestamp source to string (if it's not a string yet).
	s, err := getString(v)
	if err != nil {
		ts.logger.Debug(errTimestampConversionFailed.Error(), "err", err, "type", reflect.TypeOf(v))
		return nil, errTimestampConversionFailed
	}

	// Parse the timestamp source according to the configured format
	parsedTs, err := ts.parser(s)
	if err != nil {
		ts.logger.Debug(errTimestampParsingFailed.Error(), "err", err, "format", ts.config.Format, "value", s)

		return nil, errTimestampParsingFailed
	}

	return &parsedTs, nil
}

func (ts *timestampStage) processActionOnFailure(e Entry) Entry {
	switch ts.config.ActionOnFailure {
	case timestampActionOnFailureFudge:
		return ts.processActionOnFailureFudge(e)
	case timestampActionOnFailureSkip:
		// Nothing to do
	}

	return e
}

func (ts *timestampStage) processActionOnFailureFudge(e Entry) Entry {
	labelsStr := e.Labels.String()
	lastTimestamp, ok := ts.lastKnownTimestamps.Get(labelsStr)

	// If the last known timestamp is unknown (i.e. has not been successfully parsed yet)
	// there's nothing we can do, so we're going to keep the current timestamp
	if !ok {
		return e
	}

	// Fudge the timestamp based on the last adjusted (output) value
	entry := lastTimestamp.(timestampCacheEntry)
	e.Timestamp = entry.lastAdjusted.Add(1 * time.Nanosecond)

	// Store the fudged timestamp, so that a subsequent fudged timestamp will be 1ns after it
	ts.lastKnownTimestamps.Add(labelsStr, timestampCacheEntry{lastParsed: entry.lastParsed, lastAdjusted: e.Timestamp})
	return e
}
