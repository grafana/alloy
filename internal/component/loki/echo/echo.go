package echo

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/lokipipeline"
	"github.com/grafana/loki/pkg/push"
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

type Component struct {
	opts       component.Options
	pipelineV2 bool
	receiver   loki.LogsReceiver

	mut     sync.RWMutex
	stopped bool
}

// New creates a new loki.echo component.
func New(o component.Options, args Arguments) (*Component, error) {
	svc, err := o.GetServiceData(lokipipeline.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:       o,
		pipelineV2: svc.(lokipipeline.Data).PipelineV2Enabled(),
	}

	if c.pipelineV2 {
		c.receiver = loki.NewLogsReceiverV2(c, o.ID)
	} else {
		c.receiver = loki.NewLogsReceiver(loki.WithComponentID(o.ID))
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
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.stopped = true
	}()

	if c.pipelineV2 {
		<-ctx.Done()
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-c.receiver.Chan():
			c.printEntry(e.Labels.String(), e.Entry)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(_ component.Arguments) error {
	return nil
}

func (c *Component) Consume(ctx context.Context, batch loki.Batch) error {
	c.mut.RLock()
	defer c.mut.RUnlock()
	if c.stopped {
		return loki.ErrConsumerStopped
	}

	for _, stream := range batch.Streams() {
		lbls := stream.Labels.String()
		for _, e := range stream.Entries {
			c.printEntry(lbls, e)
		}
	}
	return nil
}

func (c *Component) printEntry(lbls string, e push.Entry) {
	sm, err := e.StructuredMetadata.MarshalJSON()
	if err != nil {
		c.opts.Logger.Error("failed to marshal structured metadata", "error", err)
		sm = []byte("{}")
	}
	c.opts.Logger.Info("received log entry", "labels", lbls, "entry", e.Line, "entry_timestamp", e.Timestamp, "structured_metadata", string(sm))
}
