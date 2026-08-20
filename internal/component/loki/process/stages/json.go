package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"

	"github.com/jmespath-community/go-jmespath"
	json "github.com/json-iterator/go"
)

var (
	errExpressionsOrRegexRequired = errors.New("JMES expressions or regex is required")
	errCouldNotCompileJMES        = errors.New("could not compile JMES expression")
	errEmptyJSONStageConfig       = errors.New("empty json stage configuration")
	errEmptyJSONStageSource       = errors.New("empty source")
)

// JSONConfig represents a JSON Stage configuration
type JSONConfig struct {
	Expressions   map[string]string `alloy:"expressions,attr,optional"`
	Regex         string            `alloy:"regex,attr,optional"`
	Source        *string           `alloy:"source,attr,optional"`
	DropMalformed bool              `alloy:"drop_malformed,attr,optional"`
}

// validateJSONConfig validates a json config and returns a map of necessary jmespath expressions.
func validateJSONConfig(c *JSONConfig) (map[string]jmespath.JMESPath, *regexp.Regexp, error) {
	if c == nil {
		return nil, nil, errEmptyJSONStageConfig
	}

	if len(c.Expressions) == 0 && len(c.Regex) == 0 {
		return nil, nil, errExpressionsOrRegexRequired
	}

	if c.Source != nil && *c.Source == "" {
		return nil, nil, errEmptyJSONStageSource
	}

	expressions := map[string]jmespath.JMESPath{}

	for n, e := range c.Expressions {
		var err error
		jmes := e
		// If there is no expression, use the name as the expression.
		if e == "" {
			jmes = n
		}
		expressions[n], err = jmespath.Compile(jmes)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", errCouldNotCompileJMES, err)
		}
	}

	re, err := regexp.Compile(c.Regex)
	if err != nil {
		return nil, nil, err
	}

	return expressions, re, nil
}

var (
	_ Stage          = (*jsonStage)(nil)
	_ entryProcessor = (*jsonStage)(nil)
)

// newJSONStage creates a new json pipeline stage from a config.
func newJSONStage(logger *slog.Logger, cfg JSONConfig, next NextFn) (*jsonStage, error) {
	expressions, regex, err := validateJSONConfig(&cfg)

	if err != nil {
		return nil, err
	}

	return &jsonStage{
		next:        next,
		cfg:         &cfg,
		regex:       *regex,
		expressions: expressions,
		logger:      logger.With("stage", "json"),
	}, nil
}

// jsonStage sets extracted data using JMESPath expressions
type jsonStage struct {
	next        NextFn
	cfg         *JSONConfig
	regex       regexp.Regexp
	expressions map[string]jmespath.JMESPath
	logger      *slog.Logger
}

func (j *jsonStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			e, drop := j.processEntry(e)
			if drop {
				continue
			}
			out <- e
		}
	}()
	return out
}

func (j *jsonStage) process(ctx context.Context, entries []Entry) error {
	var dst int
	for _, e := range entries {
		e, drop := j.processEntry(e)
		if drop {
			continue
		}
		entries[dst] = e
		dst++
	}

	if dst == 0 {
		return nil
	}

	return j.next(ctx, entries[:dst])
}

func (j *jsonStage) processEntry(e Entry) (Entry, bool) {
	// If a source key is provided, the json stage should process it
	// from the extracted map, otherwise should fall back to the line.
	input := e.Line

	if j.cfg.Source != nil {
		source := *j.cfg.Source
		if _, ok := e.Extracted[source]; !ok {
			if debugEnabled(j.logger) {
				j.logger.Debug("source does not exist in the set of extracted values", "source", *j.cfg.Source)
			}
			return e, false
		}

		value, err := getString(e.Extracted[source])
		if err != nil {
			if debugEnabled(j.logger) {
				j.logger.Debug("failed to convert source value to string", "source", *j.cfg.Source, "err", err, "type", reflect.TypeOf(e.Extracted[source]))
			}
			return e, false
		}

		input = value
	}

	var data map[string]any

	if err := json.Unmarshal([]byte(input), &data); err != nil {
		if debugEnabled(j.logger) {
			j.logger.Debug("failed to unmarshal log line", "err", err)
		}
		return e, j.cfg.DropMalformed
	}

	for name, expr := range j.expressions {
		rawResult, err := expr.Search(data)
		if err != nil {
			if debugEnabled(j.logger) {
				j.logger.Debug("failed to search JMES expression", "err", err)
			}
			continue
		}
		value, ok := j.simplifyType(rawResult)
		if ok {
			e.Extracted[name] = value
		}
	}
	if j.regex.String() != "" {
		for key, rawValue := range data {
			if j.regex.MatchString(key) {
				value, ok := j.simplifyType(rawValue)
				if ok {
					e.Extracted[key] = value
				}
			}
		}
	}
	if debugEnabled(j.logger) {
		j.logger.Debug("extracted data debug in json stage", "extracted_data", e.Extracted)
	}
	return e, false
}

// simplifyType returns the value if it's a simple type (string, number, bool),
// otherwise, it returns it as a JSON string. If unsuccessful, the second return value is false.
func (j *jsonStage) simplifyType(value any) (any, bool) {
	switch value.(type) {
	case float64:
		return value, true
	case string:
		return value, true
	case bool:
		return value, true
	case nil:
		return nil, true
	default:
		// If the value wasn't a string or a number, marshal it back to json
		jm, err := json.Marshal(value)
		if err != nil {
			if debugEnabled(j.logger) {
				j.logger.Debug("failed to marshal complex type back to string", "err", err)
				return nil, false
			}
		}
		return string(jm), true
	}
}

func (*jsonStage) Cleanup() {}
