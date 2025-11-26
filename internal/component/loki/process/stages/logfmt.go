package stages

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-logfmt/logfmt"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
)

// Config Errors
var (
	ErrMappingOrRegexRequired = errors.New("logfmt mapping or regex is required")
	ErrEmptyLogfmtStageConfig = errors.New("empty logfmt stage configuration")
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
		return nil, nil, ErrEmptyLogfmtStageConfig
	}

	if len(c.Mapping) == 0 && len(c.Regex) == 0 {
		return nil, nil, ErrMappingOrRegexRequired
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

// logfmtStage sets extracted data using logfmt parser
type logfmtStage struct {
	cfg            *LogfmtConfig
	inverseMapping map[string]string
	regex          regexp.Regexp
	logger         log.Logger
}

// newLogfmtStage creates a new logfmt pipeline stage from a config.
func newLogfmtStage(logger log.Logger, config LogfmtConfig) (Stage, error) {
	// inverseMapping would hold the mapping in inverse which would make lookup easier.
	// To explain it simply, the key would be the key from parsed logfmt and value would be the key with which the data in extracted map would be set.
	inverseMapping, regex, err := validateLogfmtConfig(&config)
	if err != nil {
		return nil, err
	}

	return toStage(&logfmtStage{
		cfg:            &config,
		inverseMapping: inverseMapping,
		regex:          *regex,
		logger:         log.With(logger, "component", "stage", "type", "logfmt"),
	}), nil
}

// Process implements Stage
func (j *logfmtStage) Process(labels model.LabelSet, extracted map[string]interface{}, t *time.Time, entry *string) {
	// If a source key is provided, the logfmt stage should process it
	// from the extracted map, otherwise should fall back to the entry
	input := entry

	if j.cfg.Source != "" {
		if _, ok := extracted[j.cfg.Source]; !ok {
			level.Debug(j.logger).Log("msg", "source does not exist in the set of extracted values", "source", j.cfg.Source)
			return
		}

		value, err := getString(extracted[j.cfg.Source])
		if err != nil {
			level.Debug(j.logger).Log("msg", "failed to convert source value to string", "source", j.cfg.Source, "err", err, "type", reflect.TypeOf(extracted[j.cfg.Source]))
			return
		}

		input = &value
	}

	if input == nil {
		level.Debug(j.logger).Log("msg", "cannot parse a nil entry")
		return
	}
	decoder := logfmt.NewDecoder(strings.NewReader(*input))
	mappingExtractedEntriesCount := 0
	regexExtractedEntriesCount := 0
	for decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			// handle "mapping"
			mapKey, ok := j.inverseMapping[string(decoder.Key())]
			if ok {
				extracted[mapKey] = string(decoder.Value())
				mappingExtractedEntriesCount++
			} else if j.regex.String() != "" {
				// handle "regex"
				fmt.Println(j.regex.String(), string(decoder.Key()))
				if j.regex.MatchString(string(decoder.Key())) {
					extracted[string(decoder.Key())] = string(decoder.Value())
					regexExtractedEntriesCount++
				}
			}
		}
	}

	if decoder.Err() != nil {
		level.Error(j.logger).Log("msg", "failed to decode logfmt", "err", decoder.Err())
		return
	}

	if mappingExtractedEntriesCount != len(j.inverseMapping) {
		level.Debug(j.logger).Log("msg", fmt.Sprintf("found only %d out of %d configured mappings in logfmt stage", mappingExtractedEntriesCount, len(j.inverseMapping)))
	}
	if regexExtractedEntriesCount > 0 {
		level.Debug(j.logger).Log("msg", fmt.Sprintf("found %d mappings via regex in logfmt stage", regexExtractedEntriesCount))
	}
	level.Debug(j.logger).Log("msg", "extracted data debug in logfmt stage", "extracted data", fmt.Sprintf("%v", extracted))
}

// Name implements Stage
func (j *logfmtStage) Name() string {
	return StageTypeLogfmt
}
