package windowsevent

import (
	"context"
	"os"
	"path"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/loki/promtail/api"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/utils"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.windowsevent",
		Stability: featuregate.StabilityGenerallyAvailable,
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

	err := createBookmark(newArgs)
	if err != nil {
		return err
	}

	// Same as the loki.source.file sync position period
	bookmarkSyncPeriod := 10 * time.Second
	winTarget, err := NewTarget(c.opts.Logger, c.handle, nil, convertConfig(newArgs), bookmarkSyncPeriod)
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

// createBookmark will create bookmark for saving the positions file.
// If LegacyBookMark is specified and the BookmarkPath doesnt exist it will copy over the legacy bookmark to the new path.
func createBookmark(args Arguments) error {
	_, err := os.Stat(args.BookmarkPath)
	// If the bookmark path does not exist then we should check to see if
	if os.IsNotExist(err) {
		err = os.MkdirAll(path.Dir(args.BookmarkPath), 644)
		if err != nil {
			return err
		}
		// Check to see if we need to convert the legacy bookmark to a new one.
		// This will only trigger if the new bookmark path does not exist and legacy does.
		_, legacyErr := os.Stat(args.LegacyBookmarkPath)
		if legacyErr == nil {
			bb, readErr := os.ReadFile(args.LegacyBookmarkPath)
			if readErr == nil {
				_ = os.WriteFile(args.BookmarkPath, bb, 644)
			}
		} else {
			f, err := os.Create(args.BookmarkPath)
			if err != nil {
				return err
			}
			_ = f.Close()
		}
	}
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
