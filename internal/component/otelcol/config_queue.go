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
	Enabled      bool  `alloy:"enabled,attr,optional"`
	NumConsumers int   `alloy:"num_consumers,attr,optional"`
	QueueSize    int64 `alloy:"queue_size,attr,optional"`
	Blocking     bool  `alloy:"blocking,attr,optional"`

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
func (args *QueueArguments) Convert() (*otelexporterhelper.QueueConfig, error) {
	if args == nil {
		return nil, nil
	}

	q := &otelexporterhelper.QueueConfig{
		Enabled:      args.Enabled,
		NumConsumers: args.NumConsumers,
		QueueSize:    args.QueueSize,
		Blocking:     args.Blocking,
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

	return nil
}
