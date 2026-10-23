package stages

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"

	"github.com/go-logfmt/logfmt"
	"github.com/grafana/alloy/internal/component/common/regexp"
	"github.com/grafana/alloy/syntax"
)

var errMappingOrRegexRequired = errors.New("logfmt mapping or regex is required")

// LogfmtConfig represents a logfmt Stage configuration
type LogfmtConfig struct {
	Mapping map[string]string `alloy:"mapping,attr,optional"`
	Source  string            `alloy:"source,attr,optional"`
	Regex   regexp.Regexp     `alloy:"regex,attr,optional"`
}

var _ (syntax.Validator) = (*LogfmtConfig)(nil)

func (c *LogfmtConfig) Validate() error {
	if len(c.Mapping) == 0 && c.Regex.IsEmpty() {
		return errMappingOrRegexRequired
	}

	return nil
}

// reverseMapping returns an inverse mapping of configured mapping.
// Mapping inverse is done to make lookup easier. The key would be the key from parsed logfmt and
// value would be the key with which the data in extracted map would be set.
func reverseMapping(m map[string]string) map[string]string {
	inverseMapping := make(map[string]string)
	for k, v := range m {
		// if value is not set, use the key for setting data in extracted map.
		if v == "" {
			v = k
		}
		inverseMapping[v] = k
	}

	return inverseMapping
}

var _ entryProcessor = (*logfmtStage)(nil)

// newLogfmtStage creates a new logfmt pipeline stage from a config.
func newLogfmtStage(config LogfmtConfig, opts stageOpts) *logfmtStage {
	return &logfmtStage{
		next:           opts.next,
		cfg:            config,
		inverseMapping: reverseMapping(config.Mapping),
		hasRegex:       !config.Regex.IsEmpty(),
		logger:         opts.slogger.With("stage", "logfmt"),
	}
}

// logfmtStage sets extracted data using logfmt parser
type logfmtStage struct {
	next           nextFn
	cfg            LogfmtConfig
	inverseMapping map[string]string
	hasRegex       bool
	logger         *slog.Logger
}

func (j *logfmtStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = j.processEntry(entries[i])
	}
	return j.next(ctx, entries)
}

func (j *logfmtStage) processEntry(e Entry) Entry {
	// If a source key is provided, the logfmt stage should process it
	// from the extracted map, otherwise should fall back to the line.
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
			if j.hasRegex && j.cfg.Regex.MatchString(key) {
				e.Extracted[key] = string(decoder.Value())
				regexExtractedEntriesCount++
			}
		}
	}

	if decoder.Err() != nil {
		j.logger.Debug("failed to decode logfmt", "err", decoder.Err())
		return e
	}

	if debugEnabled(j.logger) {
		if mappingExtractedEntriesCount != len(j.inverseMapping) {
			j.logger.Debug("found only some configured mappings in logfmt stage", "found", mappingExtractedEntriesCount, "configured", len(j.inverseMapping))
		}

		if regexExtractedEntriesCount > 0 {
			j.logger.Debug("found some mappings via regex in logfmt stage", "found", regexExtractedEntriesCount)
		}

		j.logger.Debug("extracted data debug in logfmt stage", "extracted_data", e.Extracted)
	}

	return e
}
