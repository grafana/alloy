package windowsevent

import (
	"context"
	"os"
	"path"
	"sync"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/agent/internal/component/common/loki/utils"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.windowsevent",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

var (
	_ component.Component = (*Component)(nil)
)

// Component implements the loki.source.windowsevent component.
type Component struct {
	opts component.Options

	mut       sync.RWMutex
	args      Arguments
	target    *Target
	handle    *handler
	receivers []loki.LogsReceiver
}

type handler struct {
	handler chan api.Entry
}

func (h *handler) Chan() chan<- api.Entry {
	return h.handler
}

func (h *handler) Stop() {
	// This is a noop.
}

// New creates a new loki.source.windowsevent component.
func New(o component.Options, args Arguments) (*Component, error) {

	c := &Component{
		opts:      o,
		receivers: args.ForwardTo,
		handle:    &handler{handler: make(chan api.Entry)},
		args:      args,
	}

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		if c.target != nil {
			_ = c.target.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handle.handler:
			c.mut.RLock()
			lokiEntry := loki.Entry{
				Labels: entry.Labels,
				Entry:  entry.Entry,
			}
			for _, receiver := range c.receivers {
				receiver.Chan() <- lokiEntry
			}
			c.mut.RUnlock()
		}
	}

}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	// If no bookmark specified create one in the datapath.
	if newArgs.BookmarkPath == "" {
		newArgs.BookmarkPath = path.Join(c.opts.DataPath, "bookmark.xml")
	}

	// Create the bookmark file and parent folders if they don't exist.
	_, err := os.Stat(newArgs.BookmarkPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(path.Dir(newArgs.BookmarkPath), 644)
		if err != nil {
			return err
		}
		f, err := os.Create(newArgs.BookmarkPath)
		if err != nil {
			return err
		}
		_ = f.Close()
	}

	winTarget, err := NewTarget(c.opts.Logger, c.handle, nil, convertConfig(newArgs))
	if err != nil {
		return err
	}
	// Stop the original target.
	if c.target != nil {
		err := c.target.Stop()
		if err != nil {
			return err
		}
	}
	c.target = winTarget

	c.args = newArgs
	c.receivers = newArgs.ForwardTo
	return nil
}

func convertConfig(arg Arguments) *scrapeconfig.WindowsEventsTargetConfig {
	return &scrapeconfig.WindowsEventsTargetConfig{
		Locale:               uint32(arg.Locale),
		EventlogName:         arg.EventLogName,
		Query:                arg.XPathQuery,
		UseIncomingTimestamp: arg.UseIncomingTimestamp,
		BookmarkPath:         arg.BookmarkPath,
		PollInterval:         arg.PollInterval,
		ExcludeEventData:     arg.ExcludeEventData,
		ExcludeEventMessage:  arg.ExcludeEventMessage,
		ExcludeUserData:      arg.ExcludeUserdata,
		Labels:               utils.ToLabelSet(arg.Labels),
	}
}
