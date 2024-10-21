package types

import (
	"context"
	"time"
)

const AlloyFileVersion = "alloy.metrics.queue.v1"

type SerializerConfig struct {
	// MaxSignalsInBatch controls what the max batch size is.
	MaxSignalsInBatch uint32
	// FlushFrequency controls how often to write to disk regardless of MaxSignalsInBatch.
	FlushFrequency time.Duration
}

// Serializer handles converting a set of signals into a binary representation to be written to storage.
type Serializer interface {
	Start()
	Stop()
	SendSeries(ctx context.Context, data *TimeSeriesBinary) error
	SendMetadata(ctx context.Context, data *TimeSeriesBinary) error
	UpdateConfig(ctx context.Context, cfg SerializerConfig) error
}
