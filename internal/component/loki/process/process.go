// The code in this package is adapted/ported over from grafana/loki/clients/pkg/logentry.
//
// The last Loki commit scanned for upstream changes was 7d5475541c66a819f6f456a45f8c143a084e6831.
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
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

// TODO(thampiotr): We should reconsider which parts of this component should be exported and which should
//                  be internal before 1.0, specifically the metrics and stages configuration structures.
//					To keep the `stages` package internal, we may need to move the `converter` logic into
//					the `component/loki/process` package.

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

	// MaxForwardQueueSize controls the maximum number of log entries buffered
	// per downstream component. This prevents a slow destination from blocking
	// other destinations. Default is 100000.
	MaxForwardQueueSize int `alloy:"max_forward_queue_size,attr,optional"`

	// BlockOnFull controls behavior when a destination queue is full.
	// If false (default), log entries are dropped when the queue is full.
	// If true, the component will retry with exponential backoff, which may
	// slow down the entire pipeline but prevents data loss.
	BlockOnFull bool `alloy:"block_on_full,attr,optional"`
}

// DefaultArguments provides the default arguments for the loki.process
// component.
var DefaultArguments = Arguments{
	MaxForwardQueueSize: 100_000,
	BlockOnFull:         false,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Exports exposes the receiver that can be used to send log entries to
// loki.process.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// Component implements the loki.process component.
type Component struct {
	opts component.Options

	mut          sync.RWMutex
	receiver     loki.LogsReceiver
	processIn    chan<- loki.Entry
	processOut   chan loki.Entry
	entryHandler loki.EntryHandler
	stages       []stages.StageConfig

	fanoutMut           sync.RWMutex
	fanout              []loki.LogsReceiver
	queues              []*destinationQueue
	maxForwardQueueSize int
	blockOnFull         bool

	metrics            *forwardMetrics
	debugDataPublisher livedebugging.DebugDataPublisher
}

// Backoff constants for blocking mode, similar to Prometheus remote write.
const (
	minBackoff = 5 * time.Millisecond
	maxBackoff = 5 * time.Second
)

// destinationQueue manages a buffered queue for a single destination to ensure
// FIFO ordering while preventing a slow destination from blocking others.
type destinationQueue struct {
	receiver loki.LogsReceiver
	buffer   chan loki.Entry
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func newDestinationQueue(receiver loki.LogsReceiver, size int) *destinationQueue {
	dq := &destinationQueue{
		receiver: receiver,
		buffer:   make(chan loki.Entry, size),
		stopCh:   make(chan struct{}),
	}
	dq.wg.Add(1)
	go dq.run()
	return dq
}

func (dq *destinationQueue) run() {
	defer dq.wg.Done()
	for {
		select {
		case <-dq.stopCh:
			return
		case entry := <-dq.buffer:
			select {
			case <-dq.stopCh:
				return
			case dq.receiver.Chan() <- entry:
			}
		}
	}
}

// send attempts to queue an entry for sending without blocking.
// Returns true if queued, false if buffer is full.
func (dq *destinationQueue) send(entry loki.Entry) bool {
	select {
	case dq.buffer <- entry:
		return true
	default:
		return false
	}
}

// sendWithBackoff attempts to queue an entry, retrying with exponential backoff
// if the buffer is full. Returns true if queued, false if stopped during retry.
// The metrics parameter is used to track retry attempts.
func (dq *destinationQueue) sendWithBackoff(entry loki.Entry, metrics *forwardMetrics) bool {
	// First try without blocking
	select {
	case dq.buffer <- entry:
		return true
	default:
	}

	// Buffer is full, retry with backoff
	backoff := minBackoff
	for {
		select {
		case <-dq.stopCh:
			return false
		default:
		}

		metrics.enqueueRetriesTotal.Inc()

		select {
		case <-dq.stopCh:
			return false
		case <-time.After(backoff):
		}

		select {
		case dq.buffer <- entry:
			return true
		default:
			// Still full, increase backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (dq *destinationQueue) stop() {
	close(dq.stopCh)
	dq.wg.Wait()
}

// New creates a new loki.process component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               o,
		metrics:            newForwardMetrics(o.Registerer),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	c.receiver = loki.NewLogsReceiver(loki.WithComponentID(o.ID))
	c.processOut = make(chan loki.Entry)
	o.OnStateChange(Exports{Receiver: c.receiver})

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	handleOutShutdown := make(chan struct{})
	wgOut := &sync.WaitGroup{}
	defer func() {
		c.mut.RLock()
		if c.entryHandler != nil {
			c.entryHandler.Stop()
			// Stop handleOut only after the entryHandler has stopped.
			// If handleOut stops first, entryHandler might get stuck on a channel send.
			close(handleOutShutdown)
			wgOut.Wait()
		}
		c.mut.RUnlock()

		// Stop all destination queues
		c.fanoutMut.Lock()
		for _, q := range c.queues {
			q.stop()
		}
		c.queues = nil
		c.fanoutMut.Unlock()
	}()
	wgIn := &sync.WaitGroup{}
	wgIn.Add(1)
	go c.handleIn(ctx, wgIn)
	wgOut.Add(1)
	go c.handleOut(handleOutShutdown, wgOut)

	wgIn.Wait()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// Update fanout and queues. Each destination gets its own queue to ensure
	// FIFO ordering while preventing a slow destination from blocking others.
	// See https://github.com/grafana/alloy/issues/2194
	queueSize := newArgs.MaxForwardQueueSize
	if queueSize <= 0 {
		queueSize = DefaultArguments.MaxForwardQueueSize
	}
	c.fanoutMut.Lock()
	oldQueues := c.queues
	c.fanout = newArgs.ForwardTo
	c.maxForwardQueueSize = queueSize
	c.blockOnFull = newArgs.BlockOnFull
	c.queues = make([]*destinationQueue, len(newArgs.ForwardTo))
	for i, receiver := range newArgs.ForwardTo {
		c.queues[i] = newDestinationQueue(receiver, queueSize)
	}
	c.fanoutMut.Unlock()

	// Stop old queues after releasing the lock to avoid blocking
	for _, q := range oldQueues {
		q.stop()
	}

	// Then update the pipeline itself.
	c.mut.Lock()
	defer c.mut.Unlock()

	// We want to create a new pipeline if the config changed or if this is the
	// first load. This will allow a component with no stages to function
	// properly.
	if stagesChanged(c.stages, newArgs.Stages) || c.stages == nil {
		if c.entryHandler != nil {
			c.entryHandler.Stop()
		}

		pipeline, err := stages.NewPipeline(c.opts.Logger, newArgs.Stages, &c.opts.ID, c.opts.Registerer, c.opts.MinStability)
		if err != nil {
			return err
		}
		entryHandler := loki.NewEntryHandler(c.processOut, func() { pipeline.Cleanup() })
		c.entryHandler = pipeline.Wrap(entryHandler)
		c.processIn = c.entryHandler.Chan()
		c.stages = newArgs.Stages
	}

	return nil
}

func (c *Component) handleIn(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	componentID := livedebugging.ComponentID(c.opts.ID)
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-c.receiver.Chan():
			c.mut.RLock()
			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.LokiLog,
				0, // does not count because we count only the data that exists
				func() string {
					structured_metadata, err := entry.StructuredMetadata.MarshalJSON()
					if err != nil {
						level.Error(c.opts.Logger).Log("receiver", c.opts.ID, "error", err)
						structured_metadata = []byte("{}")
					}
					return fmt.Sprintf("[IN]: timestamp: %s, entry: %s, labels: %s, structured_metadata: %s", entry.Timestamp.Format(time.RFC3339Nano), entry.Line, entry.Labels.String(), string(structured_metadata))
				},
			))
			select {
			case <-ctx.Done():
				return
			case c.processIn <- entry.Clone():
				// TODO(@tpaschalis) Instead of calling Clone() at the
				// component's entrypoint here, we can try a copy-on-write
				// approach instead, so that the copy only gets made on the
				// first stage that needs to modify the entry's labels.
			}
			c.mut.RUnlock()
		}
	}
}

func (c *Component) handleOut(shutdownCh chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	componentID := livedebugging.ComponentID(c.opts.ID)
	for {
		select {
		case <-shutdownCh:
			return
		case entry := <-c.processOut:
			c.fanoutMut.RLock()
			queues := c.queues
			blockOnFull := c.blockOnFull
			c.fanoutMut.RUnlock()

			// The log entry is the same for every fanout,
			// so we can publish it only once.
			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.LokiLog,
				1,
				func() string {
					structured_metadata, err := entry.StructuredMetadata.MarshalJSON()
					if err != nil {
						level.Error(c.opts.Logger).Log("receiver", c.opts.ID, "error", err)
						structured_metadata = []byte("{}")
					}
					return fmt.Sprintf("[OUT]: timestamp: %s, entry: %s, labels: %s, structured_metadata: %s", entry.Timestamp.Format(time.RFC3339Nano), entry.Line, entry.Labels.String(), string(structured_metadata))
				},
			))

			// Send to each destination's queue. Each destination has its own
			// buffered queue with a dedicated worker goroutine, ensuring FIFO
			// ordering while preventing a slow destination from blocking others.
			// See https://github.com/grafana/alloy/issues/2194
			for _, q := range queues {
				var sent bool
				if blockOnFull {
					sent = q.sendWithBackoff(entry, c.metrics)
				} else {
					sent = q.send(entry)
				}
				if !sent {
					c.metrics.droppedEntriesTotal.Inc()
					level.Warn(c.opts.Logger).Log("msg", "dropping log entry because destination queue is full", "labels", entry.Labels.String())
				}
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
