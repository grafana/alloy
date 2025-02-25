package api

type liveDebuggingData struct {
	// ID of the component that created the data.
	ComponentID string `json:"componentID"`
	// Ids of the components which will consume the data.
	// This is needed for components that can export different types of data (most Otel components) to know
	// where the data should go. When left empty, the data is expected to be sent to all components consuming data
	// from the component that created it.
	TargetComponentIDs []string `json:"targetComponentIDs"`
	// Type specifies the category of data represented by the count (otel_metric, loki_log, target...).
	Type string `json:"type"`
	// Rate represents the number of events of the given Type sent per second by the component.
	Rate float64 `json:"rate"`
	// Count is the number of spans, metrics, logs that the data represent.
	Count uint64 `json:"-"`
}
