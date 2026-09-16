package client

import (
	"context"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// Consumer is an interface for consuming Loki log entries.
type Consumer interface {
	ConsumeEntry(ctx context.Context, entry loki.Entry) error
	Stop()
}

// DrainableConsumer extends Consumer with the ability to stop and drain any
// remaining entries. This is useful for graceful shutdowns, particularly when
// using write-ahead logs (WAL) where entries may be buffered and need to be
// fully processed before stopping.
type DrainableConsumer interface {
	Consumer
	StopAndDrain()
}
