package livedebugging

type FeedType string

const (
	Target           FeedType = "target"
	PrometheusMetric FeedType = "prometheus_metric"
	LokiLog          FeedType = "loki_log"
	OtelMetric       FeedType = "otel_metric"
	OtelLog          FeedType = "otel_log"
	OtelTrace        FeedType = "otel_trace"
)

type FeedOption func(*Feed)

func WithTargetComponentIDs(ids []string) FeedOption {
	return func(f *Feed) {
		f.TargetComponentIDs = ids
	}
}

type Feed struct {
	// ID of the component that created the feed.
	ComponentID ComponentID
	// Ids of the components which will consume the Feed data.
	// This is needed for components that can export different types of data (most Otel components) to know
	// where the Feed should go. When left empty, the Feed is expected to be sent to all components consuming data
	// from the component that created it.
	TargetComponentIDs []string
	Type               FeedType
	// Count is the number of spans, metrics, logs that the Feed represent.
	Count uint64
	// The data string is passed as a function to only compute the string if needed.
	DataFunc func() string
}

func NewFeed(componentID ComponentID, feedType FeedType, count uint64, dataFunc func() string, opts ...FeedOption) *Feed {
	feed := &Feed{
		ComponentID: componentID,
		Type:        feedType,
		Count:       count,
		DataFunc:    dataFunc,
	}

	for _, opt := range opts {
		opt(feed)
	}

	return feed
}
