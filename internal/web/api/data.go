package api

type feed struct {
	// ID of the component that created the feed.
	ComponentID string `json:"componentID"`
	// Ids of the components which will consume the Feed data.
	// This is needed for components that can export different types of data (most Otel components) to know
	// where the Feed should go. When left empty, the Feed is expected to be sent to all components consuming data
	// from the component that created it.
	TargetComponentIDs []string `json:"targetComponentIDs"`
	// Type specifies the category of data represented by the count (otel_metric, loki_log, target...).
	Type string `json:"type"`
	// Count is the number of spans, metrics, logs that the Feed represent.
	Count uint64 `json:"count"`
}
