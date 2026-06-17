package echo

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.echo",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.echo
// component.
type Arguments struct{}

// Exports holds the values exported by the loki.echo component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// DefaultArguments defines the default settings for log scraping.
var DefaultArguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

var (
	_ loki.Consumer       = (*Component)(nil)
	_ component.Component = (*Component)(nil)
)

// Component implements the loki.source.file component.
type Component struct {
	opts component.Options

	mut      sync.RWMutex
	args     Arguments
	receiver loki.LogsReceiver
}

// New creates a new loki.echo component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:     o,
		receiver: loki.NewLogsReceiver(loki.WithComponentID(o.ID)),
	}

	// Call to Update() once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: c.receiver})

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			_ = c.ConsumeEntry(ctx, entry)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	return nil
}

func (c *Component) Consume(ctx context.Context, batch loki.Batch) error {
	batch.ConsumeStreams(func(stream loki.Stream, created int64) bool {
		logger := c.opts.Logger.With("labels", stream.Labels.String())
		for _, e := range stream.Entries {
			sm, err := e.StructuredMetadata.MarshalJSON()
			if err != nil {
				logger.Error("failed to marshal structured metadata", "error", err)
				sm = []byte("{}")
			}
			logger.Info("received log entry", "entry", e.Line, "entry_timestamp", e.Timestamp, "structured_metadata", string(sm))
		}
		return true
	})
	return nil
}

// TODO: Remove this when we have moved over to batching.
func (c *Component) ConsumeEntry(ctx context.Context, entry loki.Entry) error {
	structured_metadata, err := entry.StructuredMetadata.MarshalJSON()
	if err != nil {
		c.opts.Logger.Error("failed to marshal structured metadata", "error", err)
		structured_metadata = []byte("{}")
	}
	c.opts.Logger.Info("received log entry", "entry", entry.Line, "entry_timestamp", entry.Timestamp, "labels", entry.Labels.String(), "structured_metadata", string(structured_metadata))
	return nil
}

func (c *Component) String() string {
	return c.opts.ID + ".receiver"
}
