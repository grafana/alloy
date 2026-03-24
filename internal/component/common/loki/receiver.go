package loki

import "sync"

// LogReceiverOption is an option argument passed to NewLogsReceiver.
type LogReceiverOption func(*logsReceiver)

func WithChannel(c chan Entry) LogReceiverOption {
	return func(l *logsReceiver) {
		l.entries = c
	}
}

func WithComponentID(id string) LogReceiverOption {
	return func(l *logsReceiver) {
		l.componentID = id
	}
}

// LogsReceiver is an interface providing `chan Entry` which is used for component
// communication.
type LogsReceiver interface {
	Chan() chan Entry
}

type logsReceiver struct {
	entries     chan Entry
	componentID string
}

func (l *logsReceiver) Chan() chan Entry {
	return l.entries
}

func (l *logsReceiver) String() string {
	return l.componentID + ".receiver"
}

func NewLogsReceiver(opts ...LogReceiverOption) LogsReceiver {
	l := &logsReceiver{}

	for _, o := range opts {
		o(l)
	}

	if l.entries == nil {
		l.entries = make(chan Entry)
	}

	return l
}

// LogsBatchReceiver is an interface providing `chan []Entry`. This should be used when
// multiple entries need to be sent over a channel.
type LogsBatchReceiver interface {
	Chan() chan []Entry
}

func NewLogsBatchReceiver() LogsBatchReceiver {
	return &logsBatchReceiver{c: make(chan []Entry)}
}

type logsBatchReceiver struct {
	c chan []Entry
}

func (l *logsBatchReceiver) Chan() chan []Entry {
	return l.c
}

func NewCollectingBatchReceiver() *CollectingBatchReceiver {
	c := &CollectingBatchReceiver{
		entries: make(chan []Entry),
	}
	c.wg.Go(func() {
		for batch := range c.entries {
			c.mtx.Lock()
			c.received = append(c.received, batch...)
			c.mtx.Unlock()
		}
	})
	return c
}

// CollectingBatchReceiver is a LogsBatchReceiver that will
// collect all received entries so it can later be inspected.
// Used in tests.
type CollectingBatchReceiver struct {
	entries  chan []Entry
	received []Entry
	mtx      sync.Mutex
	wg       sync.WaitGroup
	once     sync.Once
}

func (c *CollectingBatchReceiver) Chan() chan []Entry {
	return c.entries
}

func (c *CollectingBatchReceiver) Received() []Entry {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	cpy := make([]Entry, len(c.received))
	copy(cpy, c.received)
	return cpy
}

func (c *CollectingBatchReceiver) Clear() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.received = []Entry{}
}

func (c *CollectingBatchReceiver) Stop() {
	c.once.Do(func() { close(c.entries) })
	c.wg.Wait()
}
