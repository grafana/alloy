package client

import "github.com/grafana/alloy/internal/component/common/loki"

// Consumer is an interface for consuming Loki log entries. It provides a channel
// to send entries to and a method to stop the consumer.
type Consumer interface {
	Chan() chan<- loki.Entry
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
