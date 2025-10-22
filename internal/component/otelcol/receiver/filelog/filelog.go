// Package filelog provides an otelcol.receiver.filelog component.
package filelog

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/internal/textutils"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/hashicorp/go-multierror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	stanzainputfilelog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.filelog",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := filelogreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.filelog component.
type Arguments struct {
	MatchCriteria           MatchCriteria            `alloy:",squash"`
	PollInterval            time.Duration            `alloy:"poll_interval,attr,optional"`
	MaxConcurrentFiles      int                      `alloy:"max_concurrent_files,attr,optional"`
	MaxBatches              int                      `alloy:"max_batches,attr,optional"`
	StartAt                 string                   `alloy:"start_at,attr,optional"`
	FingerprintSize         units.Base2Bytes         `alloy:"fingerprint_size,attr,optional"`
	MaxLogSize              units.Base2Bytes         `alloy:"max_log_size,attr,optional"`
	Encoding                string                   `alloy:"encoding,attr,optional"`
	FlushPeriod             time.Duration            `alloy:"force_flush_period,attr,optional"`
	DeleteAfterRead         bool                     `alloy:"delete_after_read,attr,optional"`
	IncludeFileRecordNumber bool                     `alloy:"include_file_record_number,attr,optional"`
	Compression             string                   `alloy:"compression,attr,optional"`
	AcquireFSLock           bool                     `alloy:"acquire_fs_lock,attr,optional"`
	MultilineConfig         *otelcol.MultilineConfig `alloy:"multiline,block,optional"`
	TrimConfig              *otelcol.TrimConfig      `alloy:",squash"`
	Header                  *HeaderConfig            `alloy:"header,block,optional"`
	Resolver                Resolver                 `alloy:",squash"`

	Attributes map[string]string `alloy:"attributes,attr,optional"`
	Resource   map[string]string `alloy:"resource,attr,optional"`

	Operators     []otelcol.Operator             `alloy:"operators,attr,optional"`
	ConsumerRetry otelcol.ConsumerRetryArguments `alloy:"retry_on_failure,block,optional"`

	// Storage is a binding to an otelcol.storage.* component extension which handles
	// reading and writing state.
	Storage *extension.ExtensionHandler `alloy:"storage,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

type HeaderConfig struct {
	Pattern           string             `alloy:"pattern,attr"`
	MetadataOperators []otelcol.Operator `alloy:"metadata_operators,attr"`
}

type Resolver struct {
	IncludeFileName           bool `alloy:"include_file_name,attr,optional"`
	IncludeFilePath           bool `alloy:"include_file_path,attr,optional"`
	IncludeFileNameResolved   bool `alloy:"include_file_name_resolved,attr,optional"`
	IncludeFilePathResolved   bool `alloy:"include_file_path_resolved,attr,optional"`
	IncludeFileOwnerName      bool `alloy:"include_file_owner_name,attr,optional"`
	IncludeFileOwnerGroupName bool `alloy:"include_file_owner_group_name,attr,optional"`
}

type MatchCriteria struct {
	Include []string `alloy:"include,attr"`
	Exclude []string `alloy:"exclude,attr,optional"`

	ExcludeOlderThan time.Duration     `alloy:"exclude_older_than,attr,optional"`
	OrderingCriteria *OrderingCriteria `alloy:"ordering_criteria,block,optional"`
}

type OrderingCriteria struct {
	Regex   string `alloy:"regex,attr,optional"`
	TopN    int    `alloy:"top_n,attr,optional"`
	SortBy  []Sort `alloy:"sort_by,block"`
	GroupBy string `alloy:"group_by,attr,optional"`
}

type Sort struct {
	SortType  string `alloy:"sort_type,attr,optional"`
	RegexKey  string `alloy:"regex_key,attr,optional"`
	Ascending bool   `alloy:"ascending,attr,optional"`

	// Timestamp only
	Layout   string `alloy:"layout,attr,optional"`
	Location string `alloy:"location,attr,optional"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Output:      &otelcol.ConsumerArguments{},
		StartAt:     "end",
		FlushPeriod: 500 * time.Millisecond,
		Encoding:    "utf-8",
		Resolver: Resolver{
			IncludeFileName: true,
		},
		PollInterval:       200 * time.Millisecond,
		FingerprintSize:    units.KiB,
		MaxLogSize:         units.MiB,
		MaxConcurrentFiles: 1024,
	}
	args.DebugMetrics.SetToDefault()
	args.ConsumerRetry.SetToDefault()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	c := stanzainputfilelog.NewConfig()

	def := filelogreceiver.ReceiverType{}.CreateDefaultConfig()
	cfg := def.(*filelogreceiver.FileLogConfig)
	cfg.InputConfig = *c

	// consumerretry package is stanza internal so we can't just Convert
	cfg.RetryOnFailure.Enabled = args.ConsumerRetry.Enabled
	cfg.RetryOnFailure.InitialInterval = args.ConsumerRetry.InitialInterval
	cfg.RetryOnFailure.MaxInterval = args.ConsumerRetry.MaxInterval
	cfg.RetryOnFailure.MaxElapsedTime = args.ConsumerRetry.MaxElapsedTime

	for _, op := range args.Operators {
		converted, err := op.Convert()
		if err != nil {
			return nil, err
		}
		cfg.Operators = append(cfg.Operators, *converted)
	}

	cfg.InputConfig.PollInterval = args.PollInterval
	cfg.InputConfig.MaxConcurrentFiles = args.MaxConcurrentFiles
	cfg.InputConfig.MaxBatches = args.MaxBatches
	cfg.InputConfig.StartAt = args.StartAt
	cfg.InputConfig.FingerprintSize = helper.ByteSize(args.FingerprintSize)
	cfg.InputConfig.MaxLogSize = helper.ByteSize(args.MaxLogSize)
	cfg.InputConfig.Encoding = args.Encoding
	cfg.InputConfig.FlushPeriod = args.FlushPeriod
	cfg.InputConfig.DeleteAfterRead = args.DeleteAfterRead
	cfg.InputConfig.IncludeFileRecordNumber = args.IncludeFileRecordNumber
	cfg.InputConfig.Compression = args.Compression
	cfg.InputConfig.AcquireFSLock = args.AcquireFSLock

	if len(args.Attributes) > 0 {
		cfg.InputConfig.Attributes = make(map[string]helper.ExprStringConfig, len(args.Attributes))
		for k, v := range args.Attributes {
			cfg.InputConfig.Attributes[k] = helper.ExprStringConfig(v)
		}
	}

	if len(args.Resource) > 0 {
		cfg.InputConfig.Resource = make(map[string]helper.ExprStringConfig, len(args.Resource))

		for k, v := range args.Resource {
			cfg.InputConfig.Resource[k] = helper.ExprStringConfig(v)
		}
	}

	if split := args.MultilineConfig.Convert(); split != nil {
		cfg.InputConfig.SplitConfig = *split
	}
	if trim := args.TrimConfig.Convert(); trim != nil {
		cfg.InputConfig.TrimConfig = *trim
	}

	if args.Header != nil {
		cfg.InputConfig.Header = &fileconsumer.HeaderConfig{
			Pattern: args.Header.Pattern,
		}
		for _, op := range args.Header.MetadataOperators {
			converted, err := op.Convert()
			if err != nil {
				return nil, err
			}
			cfg.InputConfig.Header.MetadataOperators = append(cfg.InputConfig.Header.MetadataOperators, *converted)
		}
	}

	cfg.InputConfig.Resolver = attrs.Resolver(args.Resolver)

	cfg.InputConfig.Criteria.Include = args.MatchCriteria.Include
	cfg.InputConfig.Criteria.Exclude = args.MatchCriteria.Exclude
	cfg.InputConfig.Criteria.ExcludeOlderThan = args.MatchCriteria.ExcludeOlderThan
	if args.MatchCriteria.OrderingCriteria != nil {
		cfg.InputConfig.Criteria.OrderingCriteria.Regex = args.MatchCriteria.OrderingCriteria.Regex
		cfg.InputConfig.Criteria.OrderingCriteria.TopN = args.MatchCriteria.OrderingCriteria.TopN
		cfg.InputConfig.Criteria.OrderingCriteria.GroupBy = args.MatchCriteria.OrderingCriteria.GroupBy

		for _, s := range args.MatchCriteria.OrderingCriteria.SortBy {
			cfg.InputConfig.Criteria.OrderingCriteria.SortBy = append(cfg.InputConfig.Criteria.OrderingCriteria.SortBy, matcher.Sort{
				SortType:  s.SortType,
				RegexKey:  s.RegexKey,
				Ascending: s.Ascending,
				Layout:    s.Layout,
				Location:  s.Location,
			})
		}
	}

	// Configure storage if args.Storage is set.
	if args.Storage != nil {
		if args.Storage.Extension == nil {
			return nil, fmt.Errorf("missing storage extension")
		}

		cfg.StorageID = &args.Storage.ID
	}

	return cfg, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Storage != nil {
		m[args.Storage.ID] = args.Storage.Extension
	}
	return m
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	var errs error
	for _, op := range args.Operators {
		_, err := op.Convert()
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("failed to parse 'operator': %w", err))
		}
	}

	if args.MaxConcurrentFiles < 1 {
		errs = multierror.Append(errs, errors.New("'max_concurrent_files' must be positive"))
	}

	if args.MaxBatches < 0 {
		errs = multierror.Append(errs, errors.New("'max_batches' must not be negative"))
	}

	if args.StartAt == "end" && args.DeleteAfterRead {
		errs = multierror.Append(errs, errors.New("'delete_after_read' cannot be used with 'start_at = end'"))
	}

	_, err := textutils.LookupEncoding(args.Encoding)
	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("invalid 'encoding': %w", err))
	}

	if args.MatchCriteria.OrderingCriteria != nil {
		if args.MatchCriteria.OrderingCriteria.TopN < 0 {
			errs = multierror.Append(errs, errors.New("'top_n' must not be negative"))
		}

		for _, s := range args.MatchCriteria.OrderingCriteria.SortBy {
			if !slices.Contains([]string{"timestamp", "numeric", "lexicographic", "mtime"}, s.SortType) {
				errs = multierror.Append(errs, fmt.Errorf("invalid 'sort_type': %s", s.SortType))
			}
		}
	}

	if args.Compression != "" && args.Compression != "gzip" && args.Compression != "auto" {
		errs = multierror.Append(errs, fmt.Errorf("invalid 'compression' type: %s", args.Compression))
	}

	if args.PollInterval < 0 {
		errs = multierror.Append(errs, errors.New("'poll_interval' must not be negative"))
	}

	if args.FingerprintSize < 0 {
		errs = multierror.Append(errs, errors.New("'fingerprint_size' must not be negative"))
	}

	if args.MaxLogSize < 0 {
		errs = multierror.Append(errs, errors.New("'max_log_size' must not be negative"))
	}

	if args.FlushPeriod < 0 {
		errs = multierror.Append(errs, errors.New("'force_flush_period' must not be negative"))
	}

	if args.MatchCriteria.ExcludeOlderThan < 0 {
		errs = multierror.Append(errs, errors.New("'exclude_older_than' must not be negative"))
	}

	if args.MultilineConfig != nil {
		if err := args.MultilineConfig.Validate(); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("invalid 'multiline': %w", err))
		}
	}

	if args.Header != nil {
		if len(args.Header.MetadataOperators) == 0 {
			errs = multierror.Append(errs, errors.New("'header' requires at least one 'metadata_operator'"))
		} else {
			for _, op := range args.Header.MetadataOperators {
				_, err := op.Convert()
				if err != nil {
					errs = multierror.Append(errs, fmt.Errorf("failed to parse 'metadata_operator': %w", err))
				}
			}
		}
	}

	return errs
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
