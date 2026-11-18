package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"sync"
	"time"
	_ "time/tzdata" // embed timezone data

	"github.com/hashicorp/golang-lru/v2/simplelru"
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

// defaultTimestampConfig holds the default values for TimestampConfig.
var defaultTimestampConfig = TimestampConfig{
	ActionOnFailure:            timestampActionOnFailureDefault,
	ActionOnDuplicateTimestamp: timestampActionOnDuplicateTimestampDefault,
}

func (cfg *TimestampConfig) SetToDefault() {
	*cfg = defaultTimestampConfig
}

func (cfg *TimestampConfig) Validate() error {
	if cfg.Source == "" {
		return errTimestampSourceRequired
	}
	if cfg.Format == "" {
		return errTimestampFormatRequired
	}
	if cfg.Location != nil {
		if _, err := time.LoadLocation(*cfg.Location); err != nil {
			return fmt.Errorf("%v: %w", errInvalidLocation, err)
		}
	}
	if !slices.Contains(timestampActionOnFailureOptions, cfg.ActionOnFailure) {
		return fmt.Errorf(errInvalidActionOnFailure.Error(), timestampActionOnFailureOptions)
	}
	if !slices.Contains(timestampActionOnDuplicateTimestampOptions, cfg.ActionOnDuplicateTimestamp) {
		return fmt.Errorf(errInvalidActionOnDuplicateTimestamp.Error(), timestampActionOnDuplicateTimestampOptions)
	}
	return nil
}

type parser func(string) (time.Time, error)

// newTimestampParser builds a parser from the configured format, fallback
// formats and location. cfg must have passed Validate.
func newTimestampParser(cfg TimestampConfig) parser {
	var loc *time.Location
	if cfg.Location != nil {
		// Validate already confirmed the location loads.
		loc, _ = time.LoadLocation(*cfg.Location)
	}

	if len(cfg.FallbackFormats) == 0 {
		return convertDateLayout(cfg.Format, loc)
	}

	parsers := make([]parser, 0, len(cfg.FallbackFormats)+1)
	parsers = append(parsers, convertDateLayout(cfg.Format, loc))
	for _, format := range cfg.FallbackFormats {
		parsers = append(parsers, convertDateLayout(format, loc))
	}
	return func(input string) (time.Time, error) {
		originalTime, originalErr := parsers[0](input)
		if originalErr == nil {
			return originalTime, nil
		}
		for _, p := range parsers[1:] {
			if t, err := p(input); err == nil {
				return t, nil
			}
		}
		return originalTime, originalErr
	}
}

func timestampFudgeEnabled(cfg TimestampConfig) bool {
	return cfg.ActionOnFailure == timestampActionOnFailureFudge || cfg.ActionOnDuplicateTimestamp == timestampActionOnDuplicateTimestampFudge
}

var _ entryProcessor = (*timestampStage)(nil)

// newTimestampStage creates a new timestamp extraction pipeline stage.
func newTimestampStage(config TimestampConfig, opts stageOpts) *timestampStage {
	var (
		lastKnownTimestamps *simplelru.LRU[string, timestampCacheEntry]
		err                 error
	)
	if timestampFudgeEnabled(config) {
		lastKnownTimestamps, err = simplelru.NewLRU[string, timestampCacheEntry](maxLastKnownTimestampsCacheSize, nil)
		if err != nil {
			panic("invalid size passed to simplelru constructor")
		}
	}

	return &timestampStage{
		next:                opts.next,
		config:              config,
		logger:              opts.slogger.With("stage", "timestamp"),
		parser:              newTimestampParser(config),
		lastKnownTimestamps: lastKnownTimestamps,
	}
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
	next   nextFn
	config TimestampConfig
	logger *slog.Logger
	parser parser

	// mut guards lastKnownTimestamps.
	mut sync.Mutex
	// lastKnownTimestamps holds the last timestamp seen per label set.
	lastKnownTimestamps *simplelru.LRU[string, timestampCacheEntry]
}

func (ts *timestampStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = ts.processEntry(entries[i])
	}
	return ts.next(ctx, entries)
}

func (ts *timestampStage) processEntry(e Entry) Entry {
	parsedTs, err := ts.parseTimestampFromSource(e.Extracted)
	if err != nil {
		return ts.processActionOnFailure(e)
	}

	return ts.processActionOnSuccess(e, parsedTs)
}

func (ts *timestampStage) parseTimestampFromSource(extracted map[string]any) (time.Time, error) {
	var zero time.Time

	// Ensure the extracted data contains the timestamp source.
	v, ok := extracted[ts.config.Source]
	if !ok {
		ts.logger.Debug(errTimestampSourceMissing.Error())
		return zero, errTimestampSourceMissing
	}

	// Convert the timestamp source to string (if it's not a string yet).
	s, err := getString(v)
	if err != nil {
		ts.logger.Debug(errTimestampConversionFailed.Error(), "err", err, "type", reflect.TypeOf(v))
		return zero, errTimestampConversionFailed
	}

	// Parse the timestamp source according to the configured format
	parsedTs, err := ts.parser(s)
	if err != nil {
		ts.logger.Debug(errTimestampParsingFailed.Error(), "err", err, "format", ts.config.Format, "value", s)
		return zero, errTimestampParsingFailed
	}

	return parsedTs, nil
}

func (ts *timestampStage) processActionOnSuccess(e Entry, parsed time.Time) Entry {
	e.Timestamp = parsed
	if !timestampFudgeEnabled(ts.config) {
		return e
	}

	labelsStr := e.Labels.String()
	ts.mut.Lock()
	defer ts.mut.Unlock()

	// When action_on_duplicate_timestamp is fudge, ensure multiple messages with the
	// exact same parsed timestamp get distinct timestamps (lastKnown+1ns each) so
	// message order is preserved in Loki and Grafana.
	if ts.config.ActionOnDuplicateTimestamp == timestampActionOnDuplicateTimestampFudge {
		if lastTimestamp, ok := ts.lastKnownTimestamps.Get(labelsStr); ok && parsed.Equal(lastTimestamp.lastParsed) {
			e.Timestamp = lastTimestamp.lastAdjusted.Add(time.Nanosecond)
		}
	}
	ts.lastKnownTimestamps.Add(labelsStr, timestampCacheEntry{lastParsed: parsed, lastAdjusted: e.Timestamp})

	return e
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
	ts.mut.Lock()
	defer ts.mut.Unlock()
	lastTimestamp, ok := ts.lastKnownTimestamps.Get(labelsStr)

	// If the last known timestamp is unknown (i.e. has not been successfully parsed yet)
	// there's nothing we can do, so we're going to keep the current timestamp
	if !ok {
		return e
	}

	// Fudge the timestamp based on the last adjusted (output) value
	e.Timestamp = lastTimestamp.lastAdjusted.Add(time.Nanosecond)

	// Store the fudged timestamp, so that a subsequent fudged timestamp will be 1ns after it
	ts.lastKnownTimestamps.Add(labelsStr, timestampCacheEntry{lastParsed: lastTimestamp.lastParsed, lastAdjusted: e.Timestamp})
	return e
}
