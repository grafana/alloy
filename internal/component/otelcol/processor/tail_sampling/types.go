package tail_sampling

import (
	"encoding"
	"fmt"
	"strings"

	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type PolicyConfig struct {
	SharedPolicyConfig SharedPolicyConfig `alloy:",squash"`

	// Configs for defining composite policy
	CompositeConfig CompositeConfig `alloy:"composite,block,optional"`

	// Configs for defining and policy
	AndConfig AndConfig `alloy:"and,block,optional"`

	// Configs for defining not policy
	NotConfig NotConfig `alloy:"not,block,optional"`

	// Configs for defining drop policy
	DropConfig DropConfig `alloy:"drop,block,optional"`
}

type SharedPolicyConfig struct {
	Name                   string                 `alloy:"name,attr"`
	Type                   string                 `alloy:"type,attr"`
	LatencyConfig          LatencyConfig          `alloy:"latency,block,optional"`
	NumericAttributeConfig NumericAttributeConfig `alloy:"numeric_attribute,block,optional"`
	ProbabilisticConfig    ProbabilisticConfig    `alloy:"probabilistic,block,optional"`
	StatusCodeConfig       StatusCodeConfig       `alloy:"status_code,block,optional"`
	StringAttributeConfig  StringAttributeConfig  `alloy:"string_attribute,block,optional"`
	RateLimitingConfig     RateLimitingConfig     `alloy:"rate_limiting,block,optional"`
	BytesLimitingConfig    BytesLimitingConfig    `alloy:"bytes_limiting,block,optional"`
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
	UpperThresholdMs int64 `alloy:"upper_threshold_ms,attr,optional"`
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

// StatusCodeConfig holds the configurable settings to create a status code filter sampling
// policy evaluator.
type StatusCodeConfig struct {
	StatusCodes []string `alloy:"status_codes,attr"`
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

// RateLimitingConfig holds the configurable settings to create a rate limiting
// sampling policy evaluator.
type RateLimitingConfig struct {
	// SpansPerSecond sets the limit on the maximum nuber of spans that can be processed each second.
	SpansPerSecond int64 `alloy:"spans_per_second,attr"`
	// BurstCapacity sets the maximum burst capacity in spans. If not specified, defaults to 2x SpansPerSecond.
	BurstCapacity int64 `alloy:"burst_capacity,attr,optional"`
}

// BytesLimitingConfig holds the configurable settings to create a bytes limiting
// sampling policy evaluator using a token bucket algorithm.
type BytesLimitingConfig struct {
	// BytesPerSecond sets the limit on the maximum number of bytes that can be processed each second.
	BytesPerSecond int64 `alloy:"bytes_per_second,attr"`
	// BurstCapacity sets the maximum burst capacity in bytes.
	BurstCapacity int64 `alloy:"burst_capacity,attr,optional"`
}

// SpanCountConfig holds the configurable settings to create a Span Count filter sampling policy
// sampling policy evaluator
type SpanCountConfig struct {
	// Minimum number of spans in a Trace
	MinSpans int32 `alloy:"min_spans,attr"`
	MaxSpans int32 `alloy:"max_spans,attr,optional"`
}

// BooleanAttributeConfig holds the configurable settings to create a boolean attribute filter
// sampling policy evaluator.
type BooleanAttributeConfig struct {
	// Tag that the filter is going to be matching against.
	Key string `alloy:"key,attr"`
	// Value indicate the bool value, either true or false to use when matching against attribute values.
	// BooleanAttribute Policy will apply exact value match on Value
	Value bool `alloy:"value,attr"`
	// InvertMatch indicates that values must not match against attribute values.
	// If InvertMatch is true and Values is equal to 'true', all other values will be sampled except 'true'.
	// Also, if the specified Key does not match any resource or span attributes, data will be sampled.
	InvertMatch bool `alloy:"invert_match,attr,optional"`
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

type TraceStateConfig struct {
	// Tag that the filter is going to be matching against.
	Key string `alloy:"key,attr"`
	// Values indicate the set of values to use when matching against trace_state values.
	Values []string `alloy:"values,attr"`
}

// CompositeConfig holds the configurable settings to create a composite
// sampling policy evaluator.
type CompositeConfig struct {
	MaxTotalSpansPerSecond int64                      `alloy:"max_total_spans_per_second,attr"`
	PolicyOrder            []string                   `alloy:"policy_order,attr"`
	SubPolicyCfg           []CompositeSubPolicyConfig `alloy:"composite_sub_policy,block,optional"`
	RateAllocation         []RateAllocationConfig     `alloy:"rate_allocation,block,optional"`
}

// CompositeSubPolicyConfig holds the common configuration to all policies under composite policy.
type CompositeSubPolicyConfig struct {
	SharedPolicyConfig SharedPolicyConfig `alloy:",squash"`

	// Configs for and policy evaluator.
	AndConfig AndConfig `alloy:"and,block,optional"`
}

// RateAllocationConfig  used within composite policy
type RateAllocationConfig struct {
	Policy  string `alloy:"policy,attr"`
	Percent int64  `alloy:"percent,attr"`
}

type AndConfig struct {
	SubPolicyConfig []AndSubPolicyConfig `alloy:"and_sub_policy,block"`
}

type DropConfig struct {
	SubPolicyConfig []AndSubPolicyConfig `alloy:"drop_sub_policy,block"`
}

type NotConfig struct {
	SubPolicyConfig NotSubPolicyConfig `alloy:"not_sub_policy,block"`
}

// NotSubPolicyConfig holds the common configuration to the policy under the not policy.
type NotSubPolicyConfig struct {
	SharedPolicyConfig SharedPolicyConfig `alloy:",squash"`
}

// AndSubPolicyConfig holds the common configuration to all policies under and policy.
type AndSubPolicyConfig struct {
	SharedPolicyConfig SharedPolicyConfig `alloy:",squash"`
}

type DecisionCacheConfig struct {
	// SampledCacheSize specifies the size of the cache that holds the sampled trace IDs
	// This value will be the maximum amount of trace IDs that the cache can hold before overwriting previous IDs.
	// For effective use, this value should be at least an order of magnitude higher than Arguments.NumTraces.
	// If left as default 0, a no-op DecisionCache will be used.
	SampledCacheSize int `alloy:"sampled_cache_size,attr,optional"`
	// NonSampledCacheSize specifies the size of the cache that holds the non-sampled trace IDs.
	// This value will be the maximum amount of trace IDs that the cache can hold before overwriting previous IDs.
	// For effective use, this value should be at least an order of magnitude higher than Config.NumTraces.
	// If left as default 0, a no-op DecisionCache will be used.
	NonSampledCacheSize int `alloy:"non_sampled_cache_size,attr,optional"`
}
