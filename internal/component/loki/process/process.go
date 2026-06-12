package process

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.process",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.process
// component.
type Arguments struct {
	ForwardTo []loki.Consumer      `alloy:"forward_to,attr"`
	Stages    []stages.StageConfig `alloy:"stage,enum,optional"`
}

// Exports exposes the receiver that can be used to send log entries to
// loki.process.
type Exports struct {
	Receiver loki.Consumer `alloy:"receiver,attr"`
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// Component implements the loki.process component.
type Component struct {
	opts       component.Options
	processOut loki.LogsReceiver
	processIn  loki.LogsReceiver

	receiver *loki.InterceptorConsumer
	fanout   *loki.FanoutConsumer

	mut          sync.RWMutex
	stopped      bool
	entryHandler loki.EntryHandler
	stages       []stages.StageConfig

	debugDataPublisher livedebugging.DebugDataPublisher
}

// New creates a new loki.process component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               o,
		processOut:         loki.NewLogsReceiver(),
		processIn:          loki.NewLogsReceiver(),
		fanout:             loki.NewFanoutConsumer(args.ForwardTo),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	c.receiver = loki.NewInterceptorConsumer(
		c.opts.ID,
		c.fanout,
		loki.WithConsumeEntryHook(func(ctx context.Context, entry loki.Entry) (loki.Entry, bool, error) {
			c.mut.RLock()
			defer c.mut.RUnlock()

			if c.stopped {
				return entry, false, loki.ErrConsumerStopped
			}

			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				livedebugging.ComponentID(c.opts.ID),
				livedebugging.LokiLog,
				0, // does not count because we count only the data that exists
				func() string {
					structured_metadata, err := entry.StructuredMetadata.MarshalJSON()
					if err != nil {
						c.opts.Logger.Error("failed to marshal structured metadata", "error", err)
						structured_metadata = []byte("{}")
					}
					return fmt.Sprintf("[IN]: timestamp: %s, entry: %s, labels: %s, structured_metadata: %s", entry.Timestamp.Format(time.RFC3339Nano), entry.Line, entry.Labels.String(), string(structured_metadata))
				},
			))

			select {
			case <-ctx.Done():
				return entry, false, ctx.Err()
			case c.processIn.Chan() <- entry.Clone():
				// TODO(@tpaschalis) Instead of calling Clone() at the
				// component's entrypoint here, we can try a copy-on-write
				// approach instead, so that the copy only gets made on the
				// first stage that needs to modify the entry's labels.

				// FIXME(kalleep): This is a temporary hack, we "disconnect" entry from call. This lets us refactor
				// pipelines to be synchronous in followup pr. See https://github.com/grafana/alloy/issues/4953.
				return entry, false, nil
			}
		}),
	)

	// Call to Update() to create and start pipeline.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	o.OnStateChange(Exports{Receiver: c.receiver})

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		loki.Drain(c.processOut, c.fanout, loki.DefaultDrainTimeout, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			c.stopped = true

			if c.entryHandler != nil {
				c.entryHandler.Stop()
			}
		})
	}()

	loki.ConsumeAndProcess(ctx, c.processOut, c.fanout, func(e loki.Entry) (loki.Entry, bool) {
		c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
			livedebugging.ComponentID(c.opts.ID),
			livedebugging.LokiLog,
			1,
			func() string {
				structured_metadata, err := e.StructuredMetadata.MarshalJSON()
				if err != nil {
					c.opts.Logger.Error("failed to marshal structured metadata", "error", err)
					structured_metadata = []byte("{}")
				}
				return fmt.Sprintf(
					"[OUT]: timestamp: %s, entry: %s, labels: %s, structured_metadata: %s",
					e.Timestamp.Format(time.RFC3339Nano),
					e.Line,
					e.Labels.String(),
					string(structured_metadata),
				)
			},
		))

		return e, true
	})

	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	c.fanout.Update(newArgs.ForwardTo)

	// We want to create a new pipeline if the config changed or if this is the
	// first load. This will allow a component with no stages to function properly.
	if stagesChanged(c.stages, newArgs.Stages) || c.stages == nil {
		pipeline, err := stages.NewPipeline(c.opts.Logger, newArgs.Stages, c.opts.Registerer, c.opts.MinStability)
		if err != nil {
			return err
		}

		// NOTE: it is important that we only stop current pipeline if we successfully created the new one.
		if c.entryHandler != nil {
			c.entryHandler.Stop()
		}

		c.stages = newArgs.Stages
		c.entryHandler = pipeline.Start(c.processIn.Chan(), c.processOut.Chan())
	}

	return nil
}

func stagesChanged(prev, next []stages.StageConfig) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}

func (c *Component) LiveDebugging() {}
