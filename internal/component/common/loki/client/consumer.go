package client

import (
	"context"
	"errors"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// ErrWALWrite is returned by WALConsumer when an entry could not be written to the WAL.
var ErrWALWrite = errors.New("wal write failed")

// IsRetryableErr reports whether a failed ConsumeEntry is worth retrying.
func IsRetryableErr(err error) bool {
	return errors.Is(err, loki.ErrConsumerStopped) || errors.Is(err, ErrWALWrite)
}

// Consumer is an interface for consuming Loki log entries.
type Consumer interface {
	// Start prepares the consumer to accept entries. It must be called once before
	// the first ConsumeEntry.
	Start()

	// ConsumeEntry hands an entry to the consumer. It returns
	// loki.ErrConsumerStopped once Stop has been called.
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
