package livedebugging

type DataType string

const (
	Target           DataType = "target"
	PrometheusMetric DataType = "prometheus_metric"
	LokiLog          DataType = "loki_log"
	OtelMetric       DataType = "otel_metric"
	OtelLog          DataType = "otel_log"
	OtelTrace        DataType = "otel_trace"
)

type DataOption func(Data) Data

func WithTargetComponentIDs(ids []string) DataOption {
	return func(d Data) Data {
		d.TargetComponentIDs = ids
		return d
	}
}

type Data struct {
	// ID of the component that created the data.
	ComponentID ComponentID
	// Ids of the components which will consume the data.
	// This is needed for components that can export different types of data (most Otel components) to know
	// where the data should go. When left empty, the data is expected to be sent to all components consuming data
	// from the component that created it.
	TargetComponentIDs []string
	Type               DataType
	// Count is the number of spans, metrics, logs that the data represent.
	Count uint64
	// The data string is passed as a function to only compute the string if needed.
	DataFunc func() string
}

func NewData(componentID ComponentID, dataType DataType, count uint64, dataFunc func() string, opts ...DataOption) Data {
	data := Data{
		ComponentID: componentID,
		Type:        dataType,
		Count:       count,
		DataFunc:    dataFunc,
	}

	for _, opt := range opts {
		data = opt(data)
	}

	return data
}
