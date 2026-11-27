package process

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
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
	ForwardTo []loki.LogsReceiver  `alloy:"forward_to,attr"`
	Stages    []stages.StageConfig `alloy:"stage,enum,optional"`
}

// Exports exposes the receiver that can be used to send log entries to
// loki.process.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

var (
	_ loki.Consumer           = (*Component)(nil)
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// Component implements the loki.process component.
type Component struct {
	opts               component.Options
	receiver           loki.LogsReceiver
	debugDataPublisher livedebugging.DebugDataPublisher
	pipelineV2Enabled  bool

	mut    sync.RWMutex
	stages []stages.StageConfig

	// Pipeline v1.
	processOut loki.LogsReceiver
	processIn  loki.LogsReceiver
	fanout     *loki.Fanout
	pipeline   *stages.Pipeline

	// Pipeline v2.
	fanoutV2   *loki.FanoutV2
	pipelineV2 *stages.PipelineV2
	stopCtx    context.Context
	stop       context.CancelFunc
}

// New creates a new loki.process component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	svc, err := o.GetServiceData(lokipipelinev2.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               o,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
		pipelineV2Enabled:  svc.(lokipipelinev2.Data).PipelineV2Enabled(),
	}

	c.stopCtx, c.stop = context.WithCancel(context.Background())

	if c.pipelineV2Enabled {
		c.receiver = loki.NewLogsReceiverV2(c, o.ID)
		c.fanoutV2 = loki.NewFanoutV2(args.ForwardTo)
	} else {
		c.receiver = loki.NewLogsReceiver(loki.WithComponentID(o.ID))
		c.processOut = loki.NewLogsReceiver()
		c.processIn = loki.NewLogsReceiver()
		c.fanout = loki.NewFanout(args.ForwardTo)
	}

	o.OnStateChange(Exports{Receiver: c.receiver})

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		c.stop()
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	if c.pipelineV2Enabled {
		defer func() {
			c.stop()

			c.mut.Lock()
			defer c.mut.Unlock()
			c.pipelineV2.Stop()
		}()

		<-ctx.Done()
		return nil
	}

	defer func() {
		loki.Drain(c.processOut, c.fanout, loki.DefaultDrainTimeout, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			if c.pipeline != nil {
				c.pipeline.Stop()
			}
		})
	}()

	var wg sync.WaitGroup
	wg.Go(func() { c.handleIn(ctx) })

	wg.Go(func() {
		loki.ConsumeAndProcess(ctx, c.processOut, c.fanout, func(e loki.Entry) (loki.Entry, bool) {
			// The log entry is the same for every fanout,
			// so we can publish it only once.
			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				livedebugging.ComponentID(c.opts.ID),
				livedebugging.LokiLog,
				1,
				func() string {
					return formatDebugEntry("[OUT]", e.Labels, e.Entry)
				},
			))

			return e, true
		})
	})

	wg.Wait()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	if c.pipelineV2Enabled {
		return c.updateV2(newArgs)
	}

	return c.updateV1(newArgs)
}

func (c *Component) updateV1(newArgs Arguments) error {
	// We always want to update fanout but it's important to do so
	// after we have drained the existing pipeline in case one of the
	// new receivers is not running yet. This behaviour will go away with
	// https://github.com/grafana/alloy/issues/4953.
	defer c.fanout.UpdateChildren(newArgs.ForwardTo)

	// We want to create a new pipeline if the config changed or if this is the
	// first load. This will allow a component with no stages to function properly.
	if stagesChanged(c.stages, newArgs.Stages) || c.stages == nil {
		pipeline, err := stages.NewPipeline(c.opts.Logger, newArgs.Stages, c.opts.Registerer, c.opts.MinStability)
		if err != nil {
			return err
		}

		// NOTE: it is important that we only stop current pipeline if we successfully created the new one.
		if c.pipeline != nil {
			c.pipeline.Stop()
		}

		c.pipeline = pipeline
		c.stages = newArgs.Stages
		c.pipeline.Start(c.processIn.Chan(), c.processOut.Chan())
	}

	return nil
}

func (c *Component) updateV2(newArgs Arguments) error {
	c.fanoutV2.UpdateChildren(newArgs.ForwardTo)

	if stagesChanged(c.stages, newArgs.Stages) || c.pipelineV2 == nil {
		pipeline, err := stages.NewPipelineV2(c.opts.Logger, c.opts.Registerer, c.opts.MinStability, newArgs.Stages, outConsumer{c: c})
		if err != nil {
			return err
		}

		// Only stop the old pipeline once the new one was built successfully. Stop
		// it before swapping, so it has released resources the two could share,
		// such as metrics, before the new one is used.
		if c.pipelineV2 != nil {
			c.pipelineV2.Stop()
		}

		c.pipelineV2 = pipeline
		c.stages = newArgs.Stages
	}

	return nil
}

// Consume implements loki.Consumer.
func (c *Component) Consume(ctx context.Context, batch loki.Batch) error {
	// Does not count because we count only the data that exists.
	publishBatch(c.debugDataPublisher, c.opts.ID, "[IN]", 0, batch)

	for {
		c.mut.RLock()
		p := c.pipelineV2
		c.mut.RUnlock()

		if c.stopCtx.Err() != nil {
			return fmt.Errorf("%s: %w", c.opts.ID, loki.ErrConsumerStopped)
		}

		err := p.Consume(ctx, batch)
		// The batch never entered the pipeline, so it is safe to retry.
		if errors.Is(err, stages.ErrPipelineStopped) {
			continue
		}

		// The batch may have been modified, so only the source can retry it.
		if errors.Is(err, stages.ErrPipelineCancelled) {
			return fmt.Errorf("%s: %w", c.opts.ID, loki.ErrConsumerStopped)
		}
		return err
	}
}

func (c *Component) handleIn(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-c.receiver.Chan():
			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				livedebugging.ComponentID(c.opts.ID),
				livedebugging.LokiLog,
				0, // does not count because we count only the data that exists
				func() string {
					return formatDebugEntry("[IN]", entry.Labels, entry.Entry)
				},
			))

			select {
			case <-ctx.Done():
				return
			case c.processIn.Chan() <- entry.Clone():
				// TODO(@tpaschalis) Instead of calling Clone() at the
				// component's entrypoint here, we can try a copy-on-write
				// approach instead, so that the copy only gets made on the
				// first stage that needs to modify the entry's labels.
			}
		}
	}
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

// outConsumer publishes entries leaving the v2 pipeline to live debugging and
// forwards them to the fanout.
type outConsumer struct {
	c *Component
}

// Consume implements loki.Consumer.
func (o outConsumer) Consume(ctx context.Context, batch loki.Batch) error {
	publishBatch(o.c.debugDataPublisher, o.c.opts.ID, "[OUT]", 1, batch)
	return o.c.fanoutV2.Consume(ctx, batch)
}

// publishBatch publishes every entry in batch to live debugging for componentID.
func publishBatch(publisher livedebugging.DebugDataPublisher, componentID, prefix string, count uint64, batch loki.Batch) {
	if !publisher.IsActive(livedebugging.ComponentID(componentID)) {
		return
	}

	for _, stream := range batch.Streams() {
		for _, e := range stream.Entries {
			publisher.PublishIfActive(livedebugging.NewData(
				livedebugging.ComponentID(componentID),
				livedebugging.LokiLog,
				count,
				func() string {
					return formatDebugEntry(prefix, stream.Labels, e)
				},
			))
		}
	}
}

// formatDebugEntry formats e with labels as a live debugging line starting with prefix.
func formatDebugEntry(prefix string, labels model.LabelSet, e push.Entry) string {
	structuredMetadata, err := e.StructuredMetadata.MarshalJSON()
	if err != nil {
		structuredMetadata = []byte("{}")
	}
	return fmt.Sprintf(
		"%s: timestamp: %s, entry: %s, labels: %s, structured_metadata: %s",
		prefix,
		e.Timestamp.Format(time.RFC3339Nano),
		e.Line,
		labels.String(),
		string(structuredMetadata),
	)
}
