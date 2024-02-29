package otelcol

import (
	"fmt"

	otelexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
)

// QueueArguments holds shared settings for components which can queue
// requests.
type QueueArguments struct {
	Enabled      bool `river:"enabled,attr,optional"`
	NumConsumers int  `river:"num_consumers,attr,optional"`
	QueueSize    int  `river:"queue_size,attr,optional"`

	// TODO(rfratto): queues can send to persistent storage through an extension.
}

// DefaultQueueArguments holds default settings for QueueArguments.
var DefaultQueueArguments = QueueArguments{
	Enabled:      true,
	NumConsumers: 10,

	// Copied from [upstream]:
	//
	// 5000 queue elements at 100 requests/sec gives about 50 seconds of survival
	// of destination outage. This is a pretty decent value for production. Users
	// should calculate this from the perspective of how many seconds to buffer
	// in case of a backend outage and multiply that by the number of requests
	// per second.
	//
	// [upstream]: https://github.com/open-telemetry/opentelemetry-collector/blob/ff73e49f74d8fd8c57a849aa3ff23ae1940cc16a/exporter/exporterhelper/queued_retry.go#L62-L65
	QueueSize: 5000,
}

// SetToDefault implements river.Defaulter.
func (args *QueueArguments) SetToDefault() {
	*args = DefaultQueueArguments
}

// Convert converts args into the upstream type.
func (args *QueueArguments) Convert() *otelexporterhelper.QueueSettings {
	if args == nil {
		return nil
	}

	return &otelexporterhelper.QueueSettings{
		Enabled:      args.Enabled,
		NumConsumers: args.NumConsumers,
		QueueSize:    args.QueueSize,
	}
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
