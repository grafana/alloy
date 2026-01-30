package otelcol

import (
	"encoding"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/plog"
)

// MatchConfig has two optional MatchProperties:
//  1. 'include': to define what is processed by the processor.
//  2. 'exclude': to define what is excluded from the processor.
//
// If both 'include' and 'exclude' are specified, the 'include' properties are checked
// before the 'exclude' properties.
type MatchConfig struct {
	Include *MatchProperties `alloy:"include,block,optional"`
	Exclude *MatchProperties `alloy:"exclude,block,optional"`
}

// MatchProperties specifies the set of properties in a spans/log/metric to match
// against and if the input data should be included or excluded from the processor.
type MatchProperties struct {
	MatchType    string        `alloy:"match_type,attr"`
	RegexpConfig *RegexpConfig `alloy:"regexp,block,optional"`

	// Note: For spans, one of Services, SpanNames, Attributes, Resources, or Libraries must be specified with a
	// non-empty value for a valid configuration.

	// For logs, one of LogBodies, LogSeverityTexts, LogSeverity, Attributes, Resources, or Libraries must be specified with a
	// non-empty value for a valid configuration.

	// For metrics, MetricNames must be specified with a non-empty value for a valid configuration.

	// Services specify the list of items to match service name against.
	// A match occurs if the span's service name matches at least one item in this list.
	Services []string `alloy:"services,attr,optional"`

	// SpanNames specify the list of items to match span name against.
	// A match occurs if the span name matches at least one item in this list.
	SpanNames []string `alloy:"span_names,attr,optional"`

	// LogBodies is a list of strings that the LogRecord's body field must match against.
	LogBodies []string `alloy:"log_bodies,attr,optional"`

	// LogSeverityTexts is a list of strings that the LogRecord's severity text field must match against.
	LogSeverityTexts []string `alloy:"log_severity_texts,attr,optional"`

	// LogSeverity defines how to match against a log record's SeverityNumber, if defined.
	LogSeverity *LogSeverityNumberMatchProperties `alloy:"log_severity,block,optional"`

	// MetricNames is a list of strings to match metric name against.
	// A match occurs if metric name matches at least one item in the list.
	MetricNames []string `alloy:"metric_names,attr,optional"`

	// Attributes specifies the list of attributes to match against.
	// All of these attributes must match exactly for a match to occur.
	// Only match_type=strict is allowed if "attributes" are specified.
	Attributes []Attribute `alloy:"attribute,block,optional"`

	// Resources specify the list of items to match the resources against.
	// A match occurs if the data's resources match at least one item in this list.
	Resources []Attribute `alloy:"resource,block,optional"`

	// Libraries specify the list of items to match the implementation library against.
	// A match occurs if the span's implementation library matches at least one item in this list.
	Libraries []InstrumentationLibrary `alloy:"library,block,optional"`

	// SpanKinds specify the list of items to match the span kind against.
	// A match occurs if the span's span kind matches at least one item in this list.
	SpanKinds []string `alloy:"span_kinds,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *MatchProperties) Convert() (map[string]any, error) {
	if args == nil {
		return nil, nil
	}

	res := make(map[string]any)

	res["match_type"] = args.MatchType

	if args.RegexpConfig != nil {
		res["regexp"] = args.RegexpConfig.convert()
	}

	if len(args.Services) > 0 {
		res["services"] = args.Services
	}

	if len(args.SpanNames) > 0 {
		res["span_names"] = args.SpanNames
	}

	if len(args.LogBodies) > 0 {
		res["log_bodies"] = args.LogBodies
	}

	if len(args.LogSeverityTexts) > 0 {
		res["log_severity_texts"] = args.LogSeverityTexts
	}

	if args.LogSeverity != nil {
		// The Otel config's field is called "log_severity_number" because it uses a number.
		// The Alloy config's field is called just "log_severity" because it uses a a textual
		// representation of the log severity instead of a number.
		logSevNum, err := args.LogSeverity.convert()
		if err != nil {
			return nil, err
		}
		res["log_severity_number"] = logSevNum
	}

	if len(args.MetricNames) > 0 {
		res["metric_names"] = args.MetricNames
	}

	if subRes := convertAttributeSlice(args.Attributes); len(subRes) > 0 {
		res["attributes"] = subRes
	}

	if subRes := convertAttributeSlice(args.Resources); len(subRes) > 0 {
		res["resources"] = subRes
	}

	if subRes := convertInstrumentationLibrariesSlice(args.Libraries); len(subRes) > 0 {
		res["libraries"] = subRes
	}

	if len(args.SpanKinds) > 0 {
		res["span_kinds"] = args.SpanKinds
	}

	return res, nil
}

// Return an empty slice if the input slice is empty.
func convertAttributeSlice(attrs []Attribute) []any {
	attrArr := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		attrArr = append(attrArr, attr.convert())
	}
	return attrArr
}

// Return an empty slice if the input slice is empty.
func convertInstrumentationLibrariesSlice(libs []InstrumentationLibrary) []any {
	libsArr := make([]any, 0, len(libs))
	for _, lib := range libs {
		libsArr = append(libsArr, lib.convert())
	}
	return libsArr
}

type RegexpConfig struct {
	// CacheEnabled determines whether match results are LRU cached to make subsequent matches faster.
	// Cache size is unlimited unless CacheMaxNumEntries is also specified.
	CacheEnabled bool `alloy:"cache_enabled,attr,optional"`
	// CacheMaxNumEntries is the max number of entries of the LRU cache that stores match results.
	// CacheMaxNumEntries is ignored if CacheEnabled is false.
	CacheMaxNumEntries int `alloy:"cache_max_num_entries,attr,optional"`
}

func (args RegexpConfig) convert() map[string]any {
	return map[string]any{
		"cacheenabled":       args.CacheEnabled,
		"cachemaxnumentries": args.CacheMaxNumEntries,
	}
}

// Attribute specifies the attribute key and optional value to match against.
type Attribute struct {
	// Key specifies the attribute key.
	Key string `alloy:"key,attr"`

	// Values specifies the value to match against.
	// If it is not set, any value will match.
	Value any `alloy:"value,attr,optional"`
}

func (args Attribute) convert() map[string]any {
	return map[string]any{
		"key":   args.Key,
		"value": args.Value,
	}
}

// InstrumentationLibrary specifies the instrumentation library and optional version to match against.
type InstrumentationLibrary struct {
	Name string `alloy:"name,attr"`
	// version match
	//  expected actual  match
	//  nil      <blank> yes
	//  nil      1       yes
	//  <blank>  <blank> yes
	//  <blank>  1       no
	//  1        <blank> no
	//  1        1       yes
	Version *string `alloy:"version,attr,optional"`
}

func (args InstrumentationLibrary) convert() map[string]any {
	res := map[string]any{
		"name": args.Name,
	}

	if args.Version != nil {
		res["version"] = strings.Clone(*args.Version)
	}
	return res
}

// LogSeverityNumberMatchProperties defines how to match based on a log record's SeverityNumber field.
type LogSeverityNumberMatchProperties struct {
	// Min is the lowest severity that may be matched.
	// e.g. if this is plog.SeverityNumberInfo, INFO, WARN, ERROR, and FATAL logs will match.
	Min SeverityLevel `alloy:"min,attr"`

	// MatchUndefined controls whether logs with "undefined" severity matches.
	// If this is true, entries with undefined severity will match.
	MatchUndefined bool `alloy:"match_undefined,attr"`
}

func (args LogSeverityNumberMatchProperties) convert() (map[string]any, error) {
	numVal, exists := severityLevels[args.Min]
	if !exists {
		return nil, fmt.Errorf("no severity value for %q", args.Min)
	}

	return map[string]any{
		"min":             numVal,
		"match_undefined": args.MatchUndefined,
	}, nil
}

type SeverityLevel string

var (
	_ encoding.TextUnmarshaler = (*SeverityLevel)(nil)
)

// The severity levels should be in sync with "opentelemetry-collector/pdata/plog/severity_number.go"
var severityLevels = map[SeverityLevel]plog.SeverityNumber{
	"TRACE":  1,
	"TRACE2": 2,
	"TRACE3": 3,
	"TRACE4": 4,
	"DEBUG":  5,
	"DEBUG2": 6,
	"DEBUG3": 7,
	"DEBUG4": 8,
	"INFO":   9,
	"INFO2":  10,
	"INFO3":  11,
	"INFO4":  12,
	"WARN":   13,
	"WARN2":  14,
	"WARN3":  15,
	"WARN4":  16,
	"ERROR":  17,
	"ERROR2": 18,
	"ERROR3": 19,
	"ERROR4": 20,
	"FATAL":  21,
	"FATAL2": 22,
	"FATAL3": 23,
	"FATAL4": 24,
}

var severityNumbers = map[plog.SeverityNumber]SeverityLevel{
	1:  "TRACE",
	2:  "TRACE2",
	3:  "TRACE3",
	4:  "TRACE4",
	5:  "DEBUG",
	6:  "DEBUG2",
	7:  "DEBUG3",
	8:  "DEBUG4",
	9:  "INFO",
	10: "INFO2",
	11: "INFO3",
	12: "INFO4",
	13: "WARN",
	14: "WARN2",
	15: "WARN3",
	16: "WARN4",
	17: "ERROR",
	18: "ERROR2",
	19: "ERROR3",
	20: "ERROR4",
	21: "FATAL",
	22: "FATAL2",
	23: "FATAL3",
	24: "FATAL4",
}

// UnmarshalText implements encoding.TextUnmarshaler for SeverityLevel.
func (sl *SeverityLevel) UnmarshalText(text []byte) error {
	alloySevLevelStr := SeverityLevel(text)
	if _, exists := severityLevels[alloySevLevelStr]; exists {
		*sl = alloySevLevelStr
		return nil
	}
	return fmt.Errorf("unrecognized severity level %q", string(text))
}

func LookupSeverityNumber(num plog.SeverityNumber) (SeverityLevel, error) {
	if lvl, exists := severityNumbers[num]; exists {
		return lvl, nil
	}

	return "", fmt.Errorf("unrecognized severity number %q", num)
}
