// Package redaction provides an otelcol.processor.redaction component.
package redaction

import (
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.redaction",
		Stability: featuregate.StabilityExperimental,
		Exports:   otelcol.ConsumerExports{},
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return processor.New(opts, redactionprocessor.NewFactory(), args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.redaction component.
type Arguments struct {
	// AllowAllKeys disables the AllowedKeys list when true, allowing all keys.
	AllowAllKeys bool `alloy:"allow_all_keys,attr,optional"`
	// AllowedKeys is the list of allowed attribute keys. Keys not on the list are removed.
	AllowedKeys []string `alloy:"allowed_keys,attr,optional"`
	// IgnoredKeys is the list of attribute keys that pass through unchanged.
	IgnoredKeys []string `alloy:"ignored_keys,attr,optional"`
	// BlockedKeyPatterns is a list of regexes; matching attribute keys are masked.
	BlockedKeyPatterns []string `alloy:"blocked_key_patterns,attr,optional"`
	// IgnoredKeyPatterns is a list of regexes; matching attribute keys pass through unchanged.
	IgnoredKeyPatterns []string `alloy:"ignored_key_patterns,attr,optional"`
	// AllowedValues is a list of regexes; matching value substrings are left unchanged even if they also match BlockedValues.
	AllowedValues []string `alloy:"allowed_values,attr,optional"`
	// BlockedValues is a list of regexes; matching value substrings are masked after key filtering.
	BlockedValues []string `alloy:"blocked_values,attr,optional"`
	// HashFunction hashes redacted values instead of masking them with a fixed string.
	HashFunction string `alloy:"hash_function,attr,optional"`
	// HMACKey is the secret key used for HMAC hashing.
	HMACKey alloytypes.Secret `alloy:"hmac_key,attr,optional"`
	// RedactAllTypes redacts non-string attributes as well by stringifying them.
	RedactAllTypes bool `alloy:"redact_all_types,attr,optional"`
	// Summary controls the verbosity of the diagnostic attributes the processor adds.
	Summary string `alloy:"summary,attr,optional"`

	// URLSanitizer sanitizes high-cardinality URLs. Optional.
	URLSanitizer *URLSanitizerArguments `alloy:"url_sanitizer,block,optional"`
	// DBSanitizer sanitizes database queries. Optional.
	DBSanitizer *DBSanitizerArguments `alloy:"db_sanitizer,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// URLSanitizerArguments configures URL sanitization.
type URLSanitizerArguments struct {
	Enabled          bool     `alloy:"enabled,attr,optional"`
	Attributes       []string `alloy:"attributes,attr,optional"`
	SanitizeSpanName *bool    `alloy:"sanitize_span_name,attr,optional"`
}

func (a URLSanitizerArguments) convert() map[string]any {
	return map[string]any{
		"enabled":            a.Enabled,
		"attributes":         a.Attributes,
		"sanitize_span_name": a.SanitizeSpanName,
	}
}

// DBSanitizerArguments configures database query sanitization.
type DBSanitizerArguments struct {
	SanitizeSpanName *bool            `alloy:"sanitize_span_name,attr,optional"`
	SQL              DBSanitizerBlock `alloy:"sql,block,optional"`
	Redis            DBSanitizerBlock `alloy:"redis,block,optional"`
	Valkey           DBSanitizerBlock `alloy:"valkey,block,optional"`
	Memcached        DBSanitizerBlock `alloy:"memcached,block,optional"`
	Mongo            DBSanitizerBlock `alloy:"mongo,block,optional"`
	OpenSearch       DBSanitizerBlock `alloy:"opensearch,block,optional"`
	ES               DBSanitizerBlock `alloy:"es,block,optional"`
}

func (a DBSanitizerArguments) convert() map[string]any {
	return map[string]any{
		"sanitize_span_name": a.SanitizeSpanName,
		"sql":                a.SQL.convert(),
		"redis":              a.Redis.convert(),
		"valkey":             a.Valkey.convert(),
		"memcached":          a.Memcached.convert(),
		"mongo":              a.Mongo.convert(),
		"opensearch":         a.OpenSearch.convert(),
		"es":                 a.ES.convert(),
	}
}

// DBSanitizerBlock is the shared shape of every per-database sanitizer sub-block.
type DBSanitizerBlock struct {
	Enabled    bool     `alloy:"enabled,attr,optional"`
	Attributes []string `alloy:"attributes,attr,optional"`
}

func (b DBSanitizerBlock) convert() map[string]any {
	return map[string]any{
		"enabled":    b.Enabled,
		"attributes": b.Attributes,
	}
}

var (
	_ processor.Arguments = Arguments{}
	_ syntax.Validator    = (*Arguments)(nil)
	_ syntax.Defaulter    = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator. It adds a parse-time enum check on
// hash_function (mapstructure.Decode bypasses HashFunction.UnmarshalText, so the
// value is otherwise unvalidated), then delegates the remaining checks — notably
// the HMAC key requirements — to the upstream Config.Validate.
func (args *Arguments) Validate() error {
	switch redactionprocessor.HashFunction(args.HashFunction) {
	case redactionprocessor.None,
		redactionprocessor.SHA1,
		redactionprocessor.SHA3,
		redactionprocessor.MD5,
		redactionprocessor.HMACSHA256,
		redactionprocessor.HMACSHA512:
		// Valid.
	default:
		return fmt.Errorf("invalid hash_function %q, allowed values are %q, %q, %q, %q and %q",
			args.HashFunction,
			redactionprocessor.SHA1, redactionprocessor.SHA3, redactionprocessor.MD5,
			redactionprocessor.HMACSHA256, redactionprocessor.HMACSHA512)
	}

	cfg, err := args.Convert()
	if err != nil {
		return err
	}
	return cfg.(*redactionprocessor.Config).Validate()
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := map[string]any{
		"allow_all_keys":       args.AllowAllKeys,
		"allowed_keys":         args.AllowedKeys,
		"ignored_keys":         args.IgnoredKeys,
		"blocked_key_patterns": args.BlockedKeyPatterns,
		"ignored_key_patterns": args.IgnoredKeyPatterns,
		"allowed_values":       args.AllowedValues,
		"blocked_values":       args.BlockedValues,
		"hash_function":        args.HashFunction,
		"hmac_key":             string(args.HMACKey),
		"redact_all_types":     args.RedactAllTypes,
		"summary":              args.Summary,
	}
	if args.URLSanitizer != nil {
		input["url_sanitizer"] = args.URLSanitizer.convert()
	}
	if args.DBSanitizer != nil {
		input["db_sanitizer"] = args.DBSanitizer.convert()
	}

	var result redactionprocessor.Config
	if err := mapstructure.Decode(input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements processor.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements processor.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements processor.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
