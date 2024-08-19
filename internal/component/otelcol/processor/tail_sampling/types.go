package tail_sampling

import (
	"encoding"
	"fmt"
	"strings"

	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	tsp "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
)

type PolicyConfig struct {
	SharedPolicyConfig SharedPolicyConfig `alloy:",squash"`

	// Configs for defining composite policy
	CompositeConfig CompositeConfig `alloy:"composite,block,optional"`

	// Configs for defining and policy
	AndConfig AndConfig `alloy:"and,block,optional"`
}

func (policyConfig PolicyConfig) Convert() tsp.PolicyCfg {
	var otelConfig tsp.PolicyCfg

	mustDecodeMapStructure(map[string]interface{}{
		"name":              policyConfig.SharedPolicyConfig.Name,
		"type":              policyConfig.SharedPolicyConfig.Type,
		"latency":           policyConfig.SharedPolicyConfig.LatencyConfig.Convert(),
		"numeric_attribute": policyConfig.SharedPolicyConfig.NumericAttributeConfig.Convert(),
		"probabilistic":     policyConfig.SharedPolicyConfig.ProbabilisticConfig.Convert(),
		"status_code":       policyConfig.SharedPolicyConfig.StatusCodeConfig.Convert(),
		"string_attribute":  policyConfig.SharedPolicyConfig.StringAttributeConfig.Convert(),
		"rate_limiting":     policyConfig.SharedPolicyConfig.RateLimitingConfig.Convert(),
		"span_count":        policyConfig.SharedPolicyConfig.SpanCountConfig.Convert(),
		"boolean_attribute": policyConfig.SharedPolicyConfig.BooleanAttributeConfig.Convert(),
		"ottl_condition":    policyConfig.SharedPolicyConfig.OttlConditionConfig.Convert(),
		"trace_state":       policyConfig.SharedPolicyConfig.TraceStateConfig.Convert(),
		"composite":         policyConfig.CompositeConfig.Convert(),
		"and":               policyConfig.AndConfig.Convert(),
	}, &otelConfig)

	return otelConfig
}

// This cannot currently have a Convert() because tsp.sharedPolicyCfg isn't public
type SharedPolicyConfig struct {
	Name                   string                 `alloy:"name,attr"`
	Type                   string                 `alloy:"type,attr"`
	LatencyConfig          LatencyConfig          `alloy:"latency,block,optional"`
	NumericAttributeConfig NumericAttributeConfig `alloy:"numeric_attribute,block,optional"`
	ProbabilisticConfig    ProbabilisticConfig    `alloy:"probabilistic,block,optional"`
	StatusCodeConfig       StatusCodeConfig       `alloy:"status_code,block,optional"`
	StringAttributeConfig  StringAttributeConfig  `alloy:"string_attribute,block,optional"`
	RateLimitingConfig     RateLimitingConfig     `alloy:"rate_limiting,block,optional"`
	SpanCountConfig        SpanCountConfig        `alloy:"span_count,block,optional"`
	BooleanAttributeConfig BooleanAttributeConfig `alloy:"boolean_attribute,block,optional"`
	OttlConditionConfig    OttlConditionConfig    `alloy:"ottl_condition,block,optional"`
	TraceStateConfig       TraceStateConfig       `alloy:"trace_state,block,optional"`
}

// LatencyConfig holds the configurable settings to create a latency filter sampling policy
// evaluator
type LatencyConfig struct {
	// ThresholdMs in milliseconds.
	ThresholdMs int64 `alloy:"threshold_ms,attr"`
	// Upper bound in milliseconds.
	UpperThresholdmsMs int64 `alloy:"upper_threshold_ms,attr,optional"`
}

func (latencyConfig LatencyConfig) Convert() tsp.LatencyCfg {
	return tsp.LatencyCfg{
		ThresholdMs:        latencyConfig.ThresholdMs,
		UpperThresholdmsMs: latencyConfig.UpperThresholdmsMs,
	}
}

// NumericAttributeConfig holds the configurable settings to create a numeric attribute filter
// sampling policy evaluator.
type NumericAttributeConfig struct {
	// Tag that the filter is going to be matching against.
	Key string `alloy:"key,attr"`
	// MinValue is the minimum value of the attribute to be considered a match.
	MinValue int64 `alloy:"min_value,attr"`
	// MaxValue is the maximum value of the attribute to be considered a match.
	MaxValue int64 `alloy:"max_value,attr"`
	// InvertMatch indicates that values must not match against attribute values.
	// If InvertMatch is true and Values is equal to '123', all other values will be sampled except '123'.
	// Also, if the specified Key does not match any resource or span attributes, data will be sampled.
	InvertMatch bool `alloy:"invert_match,attr,optional"`
}

func (numericAttributeConfig NumericAttributeConfig) Convert() tsp.NumericAttributeCfg {
	return tsp.NumericAttributeCfg{
		Key:         numericAttributeConfig.Key,
		MinValue:    numericAttributeConfig.MinValue,
		MaxValue:    numericAttributeConfig.MaxValue,
		InvertMatch: numericAttributeConfig.InvertMatch,
	}
}

// ProbabilisticConfig holds the configurable settings to create a probabilistic
// sampling policy evaluator.
type ProbabilisticConfig struct {
	// HashSalt allows one to configure the hashing salts. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same salt all passing one layer may pass the other even if they have
	// different sampling rates, configuring different salts avoids that.
	HashSalt string `alloy:"hash_salt,attr,optional"`
	// SamplingPercentage is the percentage rate at which traces are going to be sampled. Defaults to zero, i.e.: no sample.
	// Values greater or equal 100 are treated as "sample all traces".
	SamplingPercentage float64 `alloy:"sampling_percentage,attr"`
}

func (probabilisticConfig ProbabilisticConfig) Convert() tsp.ProbabilisticCfg {
	return tsp.ProbabilisticCfg{
		HashSalt:           probabilisticConfig.HashSalt,
		SamplingPercentage: probabilisticConfig.SamplingPercentage,
	}
}

// StatusCodeConfig holds the configurable settings to create a status code filter sampling
// policy evaluator.
type StatusCodeConfig struct {
	StatusCodes []string `alloy:"status_codes,attr"`
}

func (statusCodeConfig StatusCodeConfig) Convert() tsp.StatusCodeCfg {
	return tsp.StatusCodeCfg{
		StatusCodes: statusCodeConfig.StatusCodes,
	}
}

// StringAttributeConfig holds the configurable settings to create a string attribute filter
// sampling policy evaluator.
type StringAttributeConfig struct {
	// Tag that the filter is going to be matching against.
	Key string `alloy:"key,attr"`
	// Values indicate the set of values or regular expressions to use when matching against attribute values.
	// StringAttribute Policy will apply exact value match on Values unless EnabledRegexMatching is true.
	Values []string `alloy:"values,attr"`
	// EnabledRegexMatching determines whether match attribute values by regexp string.
	EnabledRegexMatching bool `alloy:"enabled_regex_matching,attr,optional"`
	// CacheMaxSize is the maximum number of attribute entries of LRU Cache that stores the matched result
	// from the regular expressions defined in Values.
	// CacheMaxSize will not be used if EnabledRegexMatching is set to false.
	CacheMaxSize int `alloy:"cache_max_size,attr,optional"`
	// InvertMatch indicates that values or regular expressions must not match against attribute values.
	// If InvertMatch is true and Values is equal to 'acme', all other values will be sampled except 'acme'.
	// Also, if the specified Key does not match on any resource or span attributes, data will be sampled.
	InvertMatch bool `alloy:"invert_match,attr,optional"`
}

func (stringAttributeConfig StringAttributeConfig) Convert() tsp.StringAttributeCfg {
	return tsp.StringAttributeCfg{
		Key:                  stringAttributeConfig.Key,
		Values:               stringAttributeConfig.Values,
		EnabledRegexMatching: stringAttributeConfig.EnabledRegexMatching,
		CacheMaxSize:         stringAttributeConfig.CacheMaxSize,
		InvertMatch:          stringAttributeConfig.InvertMatch,
	}
}

// RateLimitingConfig holds the configurable settings to create a rate limiting
// sampling policy evaluator.
type RateLimitingConfig struct {
	// SpansPerSecond sets the limit on the maximum nuber of spans that can be processed each second.
	SpansPerSecond int64 `alloy:"spans_per_second,attr"`
}

func (rateLimitingConfig RateLimitingConfig) Convert() tsp.RateLimitingCfg {
	return tsp.RateLimitingCfg{
		SpansPerSecond: rateLimitingConfig.SpansPerSecond,
	}
}

// SpanCountConfig holds the configurable settings to create a Span Count filter sampling policy
// sampling policy evaluator
type SpanCountConfig struct {
	// Minimum number of spans in a Trace
	MinSpans int32 `alloy:"min_spans,attr"`
	MaxSpans int32 `alloy:"max_spans,attr,optional"`
}

func (spanCountConfig SpanCountConfig) Convert() tsp.SpanCountCfg {
	return tsp.SpanCountCfg{
		MinSpans: spanCountConfig.MinSpans,
		MaxSpans: spanCountConfig.MaxSpans,
	}
}

// BooleanAttributeConfig holds the configurable settings to create a boolean attribute filter
// sampling policy evaluator.
type BooleanAttributeConfig struct {
	// Tag that the filter is going to be matching against.
	Key string `alloy:"key,attr"`
	// Value indicate the bool value, either true or false to use when matching against attribute values.
	// BooleanAttribute Policy will apply exact value match on Value
	Value bool `alloy:"value,attr"`
}

func (booleanAttributeConfig BooleanAttributeConfig) Convert() tsp.BooleanAttributeCfg {
	return tsp.BooleanAttributeCfg{
		Key:   booleanAttributeConfig.Key,
		Value: booleanAttributeConfig.Value,
	}
}

// The error mode determines whether to ignore or propagate
// errors with evaluating OTTL conditions.
type ErrorMode string

const (
	// "ignore" ignores errors returned by conditions, logs them, and continues on to the next condition.
	ErrorModeIgnore ErrorMode = "ignore"
	// "silent" ignores errors returned by conditions, does not log them, and continues on to the next condition.
	ErrorModeSilent ErrorMode = "silent"
	// "propagate" causes the evaluation to be false and an error is returned. The data is dropped.
	ErrorModePropagate ErrorMode = "propagate"
)

var (
	_ syntax.Validator         = (*ErrorMode)(nil)
	_ encoding.TextUnmarshaler = (*ErrorMode)(nil)
)

// Validate implements syntax.Validator.
func (e *ErrorMode) Validate() error {
	if e == nil {
		return nil
	}

	var ottlError ottl.ErrorMode
	return ottlError.UnmarshalText([]byte(string(*e)))
}

// Convert the Alloy type to the Otel type
func (e *ErrorMode) Convert() ottl.ErrorMode {
	if e == nil || *e == "" {
		return ottl.ErrorMode("")
	}

	var ottlError ottl.ErrorMode
	err := ottlError.UnmarshalText([]byte(string(*e)))

	//TODO: Rework this to return an error instead of panicking
	if err != nil {
		panic(err)
	}

	return ottlError
}

func (e *ErrorMode) UnmarshalText(text []byte) error {
	if e == nil {
		return nil
	}

	str := ErrorMode(strings.ToLower(string(text)))
	switch str {
	case ErrorModeIgnore, ErrorModePropagate, ErrorModeSilent:
		*e = str
		return nil
	default:
		return fmt.Errorf("unknown error mode %v", str)
	}
}

// OttlConditionConfig holds the configurable setting to create a OTTL condition filter
// sampling policy evaluator.
type OttlConditionConfig struct {
	ErrorMode           ErrorMode `alloy:"error_mode,attr"`
	SpanConditions      []string  `alloy:"span,attr,optional"`
	SpanEventConditions []string  `alloy:"spanevent,attr,optional"`
}

func (ottlConditionConfig OttlConditionConfig) Convert() tsp.OTTLConditionCfg {
	return tsp.OTTLConditionCfg{
		ErrorMode:           ottlConditionConfig.ErrorMode.Convert(),
		SpanConditions:      ottlConditionConfig.SpanConditions,
		SpanEventConditions: ottlConditionConfig.SpanEventConditions,
	}
}

type TraceStateConfig struct {
	// Tag that the filter is going to be matching against.
	Key string `alloy:"key,attr"`
	// Values indicate the set of values to use when matching against trace_state values.
	Values []string `alloy:"values,attr"`
}

func (traceStateConfig TraceStateConfig) Convert() tsp.TraceStateCfg {
	return tsp.TraceStateCfg{
		Key:    traceStateConfig.Key,
		Values: traceStateConfig.Values,
	}
}

// CompositeConfig holds the configurable settings to create a composite
// sampling policy evaluator.
type CompositeConfig struct {
	MaxTotalSpansPerSecond int64                      `alloy:"max_total_spans_per_second,attr"`
	PolicyOrder            []string                   `alloy:"policy_order,attr"`
	SubPolicyCfg           []CompositeSubPolicyConfig `alloy:"composite_sub_policy,block,optional"`
	RateAllocation         []RateAllocationConfig     `alloy:"rate_allocation,block,optional"`
}

func (compositeConfig CompositeConfig) Convert() tsp.CompositeCfg {
	var otelCompositeSubPolicyCfg []tsp.CompositeSubPolicyCfg
	for _, subPolicyCfg := range compositeConfig.SubPolicyCfg {
		otelCompositeSubPolicyCfg = append(otelCompositeSubPolicyCfg, subPolicyCfg.Convert())
	}

	var otelRateAllocationCfg []tsp.RateAllocationCfg
	for _, rateAllocation := range compositeConfig.RateAllocation {
		otelRateAllocationCfg = append(otelRateAllocationCfg, rateAllocation.Convert())
	}

	return tsp.CompositeCfg{
		MaxTotalSpansPerSecond: compositeConfig.MaxTotalSpansPerSecond,
		PolicyOrder:            compositeConfig.PolicyOrder,
		SubPolicyCfg:           otelCompositeSubPolicyCfg,
		RateAllocation:         otelRateAllocationCfg,
	}
}

// CompositeSubPolicyConfig holds the common configuration to all policies under composite policy.
type CompositeSubPolicyConfig struct {
	SharedPolicyConfig SharedPolicyConfig `alloy:",squash"`

	// Configs for and policy evaluator.
	AndConfig AndConfig `alloy:"and,block,optional"`
}

func (compositeSubPolicyConfig CompositeSubPolicyConfig) Convert() tsp.CompositeSubPolicyCfg {
	var otelConfig tsp.CompositeSubPolicyCfg

	mustDecodeMapStructure(map[string]interface{}{
		"name":              compositeSubPolicyConfig.SharedPolicyConfig.Name,
		"type":              compositeSubPolicyConfig.SharedPolicyConfig.Type,
		"latency":           compositeSubPolicyConfig.SharedPolicyConfig.LatencyConfig.Convert(),
		"numeric_attribute": compositeSubPolicyConfig.SharedPolicyConfig.NumericAttributeConfig.Convert(),
		"probabilistic":     compositeSubPolicyConfig.SharedPolicyConfig.ProbabilisticConfig.Convert(),
		"status_code":       compositeSubPolicyConfig.SharedPolicyConfig.StatusCodeConfig.Convert(),
		"string_attribute":  compositeSubPolicyConfig.SharedPolicyConfig.StringAttributeConfig.Convert(),
		"rate_limiting":     compositeSubPolicyConfig.SharedPolicyConfig.RateLimitingConfig.Convert(),
		"span_count":        compositeSubPolicyConfig.SharedPolicyConfig.SpanCountConfig.Convert(),
		"boolean_attribute": compositeSubPolicyConfig.SharedPolicyConfig.BooleanAttributeConfig.Convert(),
		"ottl_condition":    compositeSubPolicyConfig.SharedPolicyConfig.OttlConditionConfig.Convert(),
		"trace_state":       compositeSubPolicyConfig.SharedPolicyConfig.TraceStateConfig.Convert(),
		"and":               compositeSubPolicyConfig.AndConfig.Convert(),
	}, &otelConfig)

	return otelConfig
}

// RateAllocationConfig  used within composite policy
type RateAllocationConfig struct {
	Policy  string `alloy:"policy,attr"`
	Percent int64  `alloy:"percent,attr"`
}

func (rateAllocationConfig RateAllocationConfig) Convert() tsp.RateAllocationCfg {
	return tsp.RateAllocationCfg{
		Policy:  rateAllocationConfig.Policy,
		Percent: rateAllocationConfig.Percent,
	}
}

type AndConfig struct {
	SubPolicyConfig []AndSubPolicyConfig `alloy:"and_sub_policy,block"`
}

func (andConfig AndConfig) Convert() tsp.AndCfg {
	var otelPolicyCfgs []tsp.AndSubPolicyCfg
	for _, subPolicyCfg := range andConfig.SubPolicyConfig {
		otelPolicyCfgs = append(otelPolicyCfgs, subPolicyCfg.Convert())
	}

	return tsp.AndCfg{
		SubPolicyCfg: otelPolicyCfgs,
	}
}

// AndSubPolicyConfig holds the common configuration to all policies under and policy.
type AndSubPolicyConfig struct {
	SharedPolicyConfig SharedPolicyConfig `alloy:",squash"`
}

func (andSubPolicyConfig AndSubPolicyConfig) Convert() tsp.AndSubPolicyCfg {
	var otelConfig tsp.AndSubPolicyCfg

	mustDecodeMapStructure(map[string]interface{}{
		"name":              andSubPolicyConfig.SharedPolicyConfig.Name,
		"type":              andSubPolicyConfig.SharedPolicyConfig.Type,
		"latency":           andSubPolicyConfig.SharedPolicyConfig.LatencyConfig.Convert(),
		"numeric_attribute": andSubPolicyConfig.SharedPolicyConfig.NumericAttributeConfig.Convert(),
		"probabilistic":     andSubPolicyConfig.SharedPolicyConfig.ProbabilisticConfig.Convert(),
		"status_code":       andSubPolicyConfig.SharedPolicyConfig.StatusCodeConfig.Convert(),
		"string_attribute":  andSubPolicyConfig.SharedPolicyConfig.StringAttributeConfig.Convert(),
		"rate_limiting":     andSubPolicyConfig.SharedPolicyConfig.RateLimitingConfig.Convert(),
		"span_count":        andSubPolicyConfig.SharedPolicyConfig.SpanCountConfig.Convert(),
		"boolean_attribute": andSubPolicyConfig.SharedPolicyConfig.BooleanAttributeConfig.Convert(),
		"ottl_condition":    andSubPolicyConfig.SharedPolicyConfig.OttlConditionConfig.Convert(),
		"trace_state":       andSubPolicyConfig.SharedPolicyConfig.TraceStateConfig.Convert(),
	}, &otelConfig)

	return otelConfig
}

// mustDecodeMapStructure decodes a map into a structure. It panics if it fails.
// This is necessary for otel types that have private fields such as sharedPolicyCfg.
func mustDecodeMapStructure(source map[string]interface{}, otelConfig interface{}) {
	err := mapstructure.Decode(source, otelConfig)

	//TODO: Rework this to return an error instead of panicking
	if err != nil {
		panic(err)
	}
}

type DecisionCacheConfig struct {
	// SampledCacheSize specifies the size of the cache that holds the sampled trace IDs
	// This value will be the maximum amount of trace IDs that the cache can hold before overwriting previous IDs.
	// For effective use, this value should be at least an order of magnitude higher than Arguments.NumTraces.
	// If left as default 0, a no-op DecisionCache will be used.
	SampledCacheSize int `alloy:"sampled_cache_size,attr,optional"`
}

func (decisionCacheConfig DecisionCacheConfig) Convert() tsp.DecisionCacheConfig {
	return tsp.DecisionCacheConfig{
		SampledCacheSize: decisionCacheConfig.SampledCacheSize,
	}
}
