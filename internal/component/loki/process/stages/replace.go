package stages

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"text/template"
)

func init() {
	for k, v := range extraFunctionMap {
		functionMap[k] = v
	}
}

// ReplaceConfig contains a regexStage configuration
type ReplaceConfig struct {
	Expression string `alloy:"expression,attr"`
	Source     string `alloy:"source,attr,optional"`
	Replace    string `alloy:"replace,attr,optional"`
}

func validateReplaceConfig(c ReplaceConfig) (*regexp.Regexp, *template.Template, error) {
	if c.Expression == "" {
		return nil, nil, errExpressionRequired
	}

	expr, err := regexp.Compile(c.Expression)
	if err != nil {
		return nil, nil, fmt.Errorf("%v: %w", errCouldNotCompileRegex, err)
	}

	templ, err := template.New("pipeline_template").Funcs(functionMap).Parse(c.Replace)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile template: %w", err)
	}

	return expr, templ, nil
}

var (
	_ Stage          = (*replaceStage)(nil)
	_ entryProcessor = (*replaceStage)(nil)
)

// newReplaceStage creates a replaceStage
func newReplaceStage(logger *slog.Logger, config ReplaceConfig, next NextFn) (*replaceStage, error) {
	expression, templ, err := validateReplaceConfig(config)
	if err != nil {
		return nil, err
	}

	return &replaceStage{
		next:       next,
		cfg:        config,
		expression: expression,
		template:   templ,
		logger:     logger.With("stage", "replace"),
	}, nil
}

// replaceStage sets extracted data using regular expressions
type replaceStage struct {
	next       NextFn
	cfg        ReplaceConfig
	expression *regexp.Regexp
	logger     *slog.Logger
	template   *template.Template
}

// Run implements Stage.
func (r *replaceStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return r.processEntry(e)
	})
}

// process implements stage.
func (r *replaceStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = r.processEntry(entries[i])
	}
	return r.next(ctx, entries)
}

// Cleanup implements Stage.
func (r *replaceStage) Cleanup() {}

func (r *replaceStage) processEntry(e Entry) Entry {
	// If a source key is provided, the replace stage should process it
	// from the extracted map, otherwise should fall back to the line.
	input := e.Line

	if r.cfg.Source != "" {
		if _, ok := e.Extracted[r.cfg.Source]; !ok {
			r.logger.Debug("source does not exist in the set of extracted values", "source", r.cfg.Source)
			return e
		}

		value, err := getString(e.Extracted[r.cfg.Source])
		if err != nil {
			r.logger.Debug("failed to convert source value to string", "source", r.cfg.Source, "err", err, "type", reflect.TypeOf(e.Extracted[r.cfg.Source]))
			return e
		}

		input = value
	}

	// Get string of matched captured groups. We will use this to extract all named captured groups
	match := r.expression.FindStringSubmatch(input)
	matchAllIndex := r.expression.FindAllStringSubmatchIndex(input, -1)

	if matchAllIndex == nil {
		r.logger.Debug("regex did not match", "input", input, "regex", r.expression)
		return e
	}

	// All extracted values will be available for templating
	td := r.getTemplateData(e.Extracted)

	result, capturedMap, err := r.getReplacedEntry(matchAllIndex, input, td)
	if err != nil {
		r.logger.Debug("failed to execute template on extracted value", "err", err)
		return e
	}

	if r.cfg.Source != "" {
		e.Extracted[r.cfg.Source] = result
	} else {
		e.Line = result
	}

	// All the named captured group will be extracted
	for i, name := range r.expression.SubexpNames() {
		if i != 0 && name != "" {
			if v, ok := capturedMap[match[i]]; ok {
				e.Extracted[name] = v
			}
		}
	}
	if debugEnabled(r.logger) {
		r.logger.Debug("extracted data debug in replace stage", "extracted_data", e.Extracted)
	}

	return e
}

func (r *replaceStage) getReplacedEntry(matchAllIndex [][]int, input string, td map[string]string) (string, map[string]string, error) {
	var result string
	previousInputEndIndex := 0
	capturedMap := make(map[string]string)
	// For a simple string like `11.11.11.11 - frank 12.12.12.12 - frank`
	// if the regex is "(\\d{2}.\\d{2}.\\d{2}.\\d{2}) - (\\S+)"
	// FindAllStringSubmatchIndex would return [[0 19 0 11 14 19] [20 37 20 31 34 37]].
	// Each inner array's first two values will be the start and end index of the entire
	// matched string and the next values will be start and end index of the matched
	// captured group. Here 0-19 is "11.11.11.11 - frank",  0-11 is "11.11.11.11" and
	// 14-19 is "frank". So, we advance by 2 index to get the next match
	for _, matchIndex := range matchAllIndex {
		for i := 2; i < len(matchIndex); i += 2 {
			if matchIndex[i] == -1 {
				continue
			}
			capturedString := input[matchIndex[i]:matchIndex[i+1]]
			buf := &bytes.Buffer{}
			td["Value"] = capturedString
			err := r.template.Execute(buf, td)
			if err != nil {
				return "", nil, err
			}
			st := buf.String()
			if previousInputEndIndex == 0 || previousInputEndIndex <= matchIndex[i] {
				result += input[previousInputEndIndex:matchIndex[i]] + st
				previousInputEndIndex = matchIndex[i+1]
			}
			capturedMap[capturedString] = st
		}
	}
	return result + input[previousInputEndIndex:], capturedMap, nil
}

func (r *replaceStage) getTemplateData(extracted map[string]any) map[string]string {
	td := make(map[string]string)
	for k, v := range extracted {
		s, err := getString(v)
		if err != nil {
			r.logger.Debug("extracted template could not be converted to a string", "err", err, "type", reflect.TypeOf(v))
			continue
		}
		td[k] = s
	}
	return td
}
