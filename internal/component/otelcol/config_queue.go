package otelcol

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/syntax"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	otelexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
)

// QueueArguments holds shared settings for components which can queue
// requests.
type QueueArguments struct {
	Enabled         bool   `alloy:"enabled,attr,optional"`
	NumConsumers    int    `alloy:"num_consumers,attr,optional"`
	QueueSize       int64  `alloy:"queue_size,attr,optional"`
	BlockOnOverflow bool   `alloy:"block_on_overflow,attr,optional"`
	Sizer           string `alloy:"sizer,attr,optional"`
	WaitForResult   bool   `alloy:"wait_for_result,attr,optional"`

	Batch *BatchConfig `alloy:"batch,block,optional"`

	// Storage is a binding to an otelcol.storage.* component extension which handles
	// reading and writing to disk
	Storage *extension.ExtensionHandler `alloy:"storage,attr,optional"`
}

var _ syntax.Defaulter = (*QueueArguments)(nil)
var _ syntax.Validator = (*QueueArguments)(nil)

// SetToDefault implements syntax.Defaulter.
func (args *QueueArguments) SetToDefault() {
	*args = QueueArguments{
		Enabled:      true,
		NumConsumers: 10,

		// Copied from [upstream](https://github.com/open-telemetry/opentelemetry-collector/blob/241334609fc47927b4a8533dfca28e0f65dad9fe/exporter/exporterhelper/queue_sender.go#L50-L53)
		//
		// By default, batches are 8192 spans, for a total of up to 8 million spans in the queue
		// This can be estimated at 1-4 GB worth of maximum memory usage
		// This default is probably still too high, and may be adjusted further down in a future release
		QueueSize: 1000,
		Sizer:     "requests",
	}
}

// Extensions returns a map of extensions to be used by the component.
func (args *QueueArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	if args == nil {
		return nil
	}
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Storage != nil {
		m[args.Storage.ID] = args.Storage.Extension
	}
	return m
}

// Convert converts args into the upstream type.
func (args *QueueArguments) Convert() (configoptional.Optional[otelexporterhelper.QueueBatchConfig], error) {
	if args == nil || !args.Enabled {
		return configoptional.None[otelexporterhelper.QueueBatchConfig](), nil
	}

	sizer, err := convertSizer(args.Sizer)
	if err != nil {
		if args.Enabled {
			return configoptional.None[otelexporterhelper.QueueBatchConfig](), err
		} else {
			// This is a workaround for components which have queue arguments,
			// but don't use them in some cases. For example, the loadbalancing exporter
			// doesn't set the queue arguments by default. This leaves the sizer empty,
			// and then the convert function complains that the sizer value is invalid.
			// If the queue is disabled then it should be ok just to set the sizer to the default value -
			// it won't make a difference anyway.
			sizer = &otelexporterhelper.RequestSizerTypeRequests
		}
	}

	batch, err := args.Batch.Convert()
	if err != nil {
		return configoptional.None[otelexporterhelper.QueueBatchConfig](), err
	}
	q := otelexporterhelper.NewDefaultQueueConfig()
	q.NumConsumers = args.NumConsumers
	q.QueueSize = args.QueueSize
	q.BlockOnOverflow = args.BlockOnOverflow
	q.Sizer = *sizer
	q.WaitForResult = args.WaitForResult
	q.Batch = batch

	// Configure storage if args.Storage is set.
	if args.Storage != nil {
		if args.Storage.Extension == nil {
			return configoptional.None[otelexporterhelper.QueueBatchConfig](), fmt.Errorf("missing storage extension")
		}

		q.StorageID = &args.Storage.ID
	}

	return configoptional.Some(q), nil
}

// Validate returns an error if args is invalid.
func (args *QueueArguments) Validate() error {
	if args == nil || !args.Enabled {
		return nil
	}

	if args.QueueSize <= 0 {
		return fmt.Errorf("queue_size must be greater than zero")
	}

	_, err := convertSizer(args.Sizer)
	if err != nil {
		return err
	}

	if args.Batch != nil {
		if args.Batch.Sizer == args.Sizer && args.Batch.MinSize > args.QueueSize {
			// Avoid situations where the queue is not able to hold any data.
			return fmt.Errorf("`min_size` must be less than or equal to `queue_size`")
		}
		if err := args.Batch.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func convertSizer(sizer string) (*otelexporterhelper.RequestSizerType, error) {
	switch sizer {
	case "bytes":
		return &otelexporterhelper.RequestSizerTypeBytes, nil
	case "items":
		return &otelexporterhelper.RequestSizerTypeItems, nil
	case "requests":
		return &otelexporterhelper.RequestSizerTypeRequests, nil
	default:
		return nil, fmt.Errorf("invalid sizer: %s", sizer)
	}
}

type BatchConfig struct {
	FlushTimeout time.Duration `alloy:"flush_timeout,attr,optional"`
	MinSize      int64         `alloy:"min_size,attr,optional"`
	MaxSize      int64         `alloy:"max_size,attr,optional"`
	Sizer        string        `alloy:"sizer,attr,optional"`
}

var _ syntax.Defaulter = (*BatchConfig)(nil)

var defaultBatchConfig = otelexporterhelper.NewDefaultQueueConfig().Batch

// SetToDefault implements syntax.Defaulter.
func (args *BatchConfig) SetToDefault() {
	*args = BatchConfig{
		FlushTimeout: 200 * time.Millisecond,
		MinSize:      2000,
		MaxSize:      3000,
		Sizer:        "items",
	}
}

// Validate returns an error if args is invalid.
func (args *BatchConfig) Validate() error {
	if args == nil {
		return nil
	}

	// Only support items or bytes sizer for batch at this moment.
	if args.Sizer != "items" && args.Sizer != "bytes" {
		return fmt.Errorf("`batch` supports only `items` or `bytes` sizer")
	}

	if args.FlushTimeout <= 0 {
		return fmt.Errorf("`flush_timeout` must be positive")
	}

	if args.MinSize < 0 {
		return fmt.Errorf("`min_size` must be non-negative")
	}

	if args.MaxSize < 0 {
		return fmt.Errorf("`max_size` must be non-negative")
	}

	if args.MaxSize > 0 && args.MaxSize < args.MinSize {
		return fmt.Errorf("`max_size` must be greater or equal to `min_size`")
	}

	return nil
}

func (args *BatchConfig) Convert() (configoptional.Optional[otelexporterhelper.BatchConfig], error) {
	if args == nil {
		return defaultBatchConfig, nil
	}

	sizer, err := convertSizer(args.Sizer)
	if err != nil {
		return configoptional.None[otelexporterhelper.BatchConfig](), err
	}

	return configoptional.Some(otelexporterhelper.BatchConfig{
		FlushTimeout: args.FlushTimeout,
		MinSize:      args.MinSize,
		MaxSize:      args.MaxSize,
		Sizer:        *sizer,
	}), nil
}
