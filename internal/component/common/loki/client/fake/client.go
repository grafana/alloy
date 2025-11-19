package fake

import (
	"sync"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// Client is a fake client used for testing.
type Client struct {
	entries  chan loki.Entry
	received []loki.Entry
	once     sync.Once
	mtx      sync.Mutex
	wg       sync.WaitGroup
	OnStop   func()
}

func NewClient(stop func()) *Client {
	c := &Client{
		OnStop:  stop,
		entries: make(chan loki.Entry),
	}
	c.wg.Go(func() {
		for e := range c.entries {
			c.mtx.Lock()
			c.received = append(c.received, e)
			c.mtx.Unlock()
		}
	})
	return c
}

// Stop implements client.Client
func (c *Client) Stop() {
	c.once.Do(func() { close(c.entries) })
	c.wg.Wait()
	c.OnStop()
}

func (c *Client) Chan() chan<- loki.Entry {
	return c.entries
}

func (c *Client) Received() []loki.Entry {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	cpy := make([]loki.Entry, len(c.received))
	copy(cpy, c.received)
	return cpy
}

// Clear is used to cleanup the buffered received entries, so the same client can be re-used between
// test cases.
func (c *Client) Clear() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.received = []loki.Entry{}
}

// LogsReceiver returns this client as a LogsReceiver, which is useful in testing.
func (c *Client) LogsReceiver() loki.LogsReceiver {
	return loki.NewLogsReceiver(loki.WithChannel(c.entries))
}
