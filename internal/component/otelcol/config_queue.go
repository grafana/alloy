package otelcol

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/syntax"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
)

// QueueArguments holds shared settings for components which can queue
// requests.
type QueueArguments struct {
	Enabled      bool   `alloy:"enabled,attr,optional"`
	NumConsumers int    `alloy:"num_consumers,attr,optional"`
	QueueSize    int64  `alloy:"queue_size,attr,optional"`
	Blocking     bool   `alloy:"blocking,attr,optional"`
	Sizer        string `alloy:"sizer,attr,optional"`

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
func (args *QueueArguments) Convert() (*otelexporterhelper.QueueBatchConfig, error) {
	if args == nil {
		return nil, nil
	}

	sizer, err := convertSizer(args.Sizer)
	if err != nil {
		if args.Enabled {
			return nil, err
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

	q := &otelexporterhelper.QueueBatchConfig{
		Enabled:      args.Enabled,
		NumConsumers: args.NumConsumers,
		QueueSize:    args.QueueSize,
		Blocking:     args.Blocking,
		Sizer:        *sizer,
	}

	// Configure storage if args.Storage is set.
	if args.Storage != nil {
		if args.Storage.Extension == nil {
			return nil, fmt.Errorf("missing storage extension")
		}

		q.StorageID = &args.Storage.ID
	}

	return q, nil
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
