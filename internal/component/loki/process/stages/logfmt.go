package stages

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-logfmt/logfmt"
)

var (
	errMappingOrRegexRequired = errors.New("logfmt mapping or regex is required")
	errEmptyLogfmtStageConfig = errors.New("empty logfmt stage configuration")
)

// LogfmtConfig represents a logfmt Stage configuration
type LogfmtConfig struct {
	Mapping map[string]string `alloy:"mapping,attr,optional"`
	Source  string            `alloy:"source,attr,optional"`
	Regex   string            `alloy:"regex,attr,optional"`
}

// validateLogfmtConfig validates a logfmt stage config and returns an inverse mapping of configured mapping.
// Mapping inverse is done to make lookup easier. The key would be the key from parsed logfmt and
// value would be the key with which the data in extracted map would be set.
func validateLogfmtConfig(c *LogfmtConfig) (map[string]string, *regexp.Regexp, error) {
	if c == nil {
		return nil, nil, errEmptyLogfmtStageConfig
	}

	if len(c.Mapping) == 0 && len(c.Regex) == 0 {
		return nil, nil, errMappingOrRegexRequired
	}

	inverseMapping := make(map[string]string)
	for k, v := range c.Mapping {
		// if value is not set, use the key for setting data in extracted map.
		if v == "" {
			v = k
		}
		inverseMapping[v] = k
	}

	re, err := regexp.Compile(c.Regex)
	if err != nil {
		return nil, nil, err
	}

	return inverseMapping, re, nil
}

var (
	_ Stage = (*logfmtStage)(nil)
	_ stage = (*logfmtStage)(nil)
)

// newLogfmtStage creates a new logfmt pipeline stage from a config.
func newLogfmtStage(logger *slog.Logger, config LogfmtConfig, next NextFn) (Stage, error) {
	// inverseMapping would hold the mapping in inverse which would make lookup easier.
	// To explain it simply, the key would be the key from parsed logfmt and value would be the key with which the data in extracted map would be set.
	inverseMapping, regex, err := validateLogfmtConfig(&config)
	if err != nil {
		return nil, err
	}

	return &logfmtStage{
		next:           next,
		cfg:            config,
		regex:          *regex,
		inverseMapping: inverseMapping,
		logger:         logger.With("stage", "logfmt"),
	}, nil
}

// logfmtStage sets extracted data using logfmt parser
type logfmtStage struct {
	next           NextFn
	cfg            LogfmtConfig
	regex          regexp.Regexp
	inverseMapping map[string]string
	logger         *slog.Logger
}

// Run implements Stage.
func (j *logfmtStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return j.processEntry(e)
	})
}

// process implements stage.
func (j *logfmtStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = j.processEntry(entries[i])
	}
	return j.next(ctx, entries)
}

// Cleanup implements Stage.
func (j *logfmtStage) Cleanup() {}

func (j *logfmtStage) processEntry(e Entry) Entry {
	// If a source key is provided, the logfmt stage should process it
	// from the extracted map, otherwise should fall back to the entry
	input := e.Line

	if j.cfg.Source != "" {
		if _, ok := e.Extracted[j.cfg.Source]; !ok {
			j.logger.Debug("source does not exist in the set of extracted values", "source", j.cfg.Source)
			return e
		}

		value, err := getString(e.Extracted[j.cfg.Source])
		if err != nil {
			j.logger.Debug("failed to convert source value to string", "source", j.cfg.Source, "err", err, "type", reflect.TypeOf(e.Extracted[j.cfg.Source]))
			return e
		}

		input = value
	}

	decoder := logfmt.NewDecoder(strings.NewReader(input))
	mappingExtractedEntriesCount := 0
	regexExtractedEntriesCount := 0
	for decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			key := string(decoder.Key())
			// handle "mapping"
			mapKey, ok := j.inverseMapping[key]
			if ok {
				e.Extracted[mapKey] = string(decoder.Value())
				mappingExtractedEntriesCount++
			}
			// handle "regex"
			if j.regex.String() != "" {
				if j.regex.MatchString(key) {
					e.Extracted[key] = string(decoder.Value())
					regexExtractedEntriesCount++
				}
			}
		}
	}

	if decoder.Err() != nil {
		j.logger.Debug("failed to decode logfmt", "err", decoder.Err())
		return e
	}

	if debugEnabled(j.logger) {
		if mappingExtractedEntriesCount != len(j.inverseMapping) {
			if mappingExtractedEntriesCount != len(j.inverseMapping) {
				j.logger.Debug("found only some configured mappings in logfmt stage", "found", mappingExtractedEntriesCount, "configured", len(j.inverseMapping))
			}

			if regexExtractedEntriesCount > 0 {
				j.logger.Debug("found some mappings via regex in logfmt stage", "found", regexExtractedEntriesCount)
			}

			j.logger.Debug("extracted data debug in logfmt stage", "extracted_data", e.Extracted)
		}
	}

	return e
}
