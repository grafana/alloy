// The code in this package is adapted/ported over from grafana/loki/clients/pkg/logentry.
//
// The last Loki commit scanned for upstream changes was 7d5475541c66a819f6f456a45f8c143a084e6831.
package process

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	"github.com/grafana/alloy/internal/featuregate"
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

	fanoutMut sync.RWMutex
	fanout    []loki.LogsReceiver

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
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	c.receiver = loki.NewLogsReceiver()
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
	defer func() {
		c.mut.RLock()
		if c.entryHandler != nil {
			c.entryHandler.Stop()
		}
		close(c.processIn)
		c.mut.RUnlock()
	}()
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go c.handleIn(ctx, wg)
	go c.handleOut(ctx, wg)

	wg.Wait()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// Update c.fanout first in case anything else fails.
	c.fanoutMut.Lock()
	c.fanout = newArgs.ForwardTo
	c.fanoutMut.Unlock()

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

		pipeline, err := stages.NewPipeline(c.opts.Logger, newArgs.Stages, &c.opts.ID, c.opts.Registerer)
		if err != nil {
			return err
		}
		c.entryHandler = loki.NewEntryHandler(c.processOut, func() { pipeline.Cleanup() })
		c.processIn = pipeline.Wrap(c.entryHandler).Chan()
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
			select {
			case <-ctx.Done():
				return
			case c.processIn <- entry.Clone():
				if c.debugDataPublisher.IsActive(componentID) {
					c.debugDataPublisher.Publish(componentID, fmt.Sprintf("[IN]: entry: %s, labels: %s", entry.Line, entry.Labels.String()))
				}
				// TODO(@tpaschalis) Instead of calling Clone() at the
				// component's entrypoint here, we can try a copy-on-write
				// approach instead, so that the copy only gets made on the
				// first stage that needs to modify the entry's labels.
			}
			c.mut.RUnlock()
		}
	}
}

func (c *Component) handleOut(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	componentID := livedebugging.ComponentID(c.opts.ID)
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-c.processOut:
			c.fanoutMut.RLock()
			fanout := c.fanout
			c.fanoutMut.RUnlock()
			for _, f := range fanout {
				select {
				case <-ctx.Done():
					return
				case f.Chan() <- entry:
					if c.debugDataPublisher.IsActive(componentID) {
						c.debugDataPublisher.Publish(componentID, fmt.Sprintf("[OUT]: entry: %s, labels: %s", entry.Line, entry.Labels.String()))
					}
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

func (c *Component) LiveDebugging(_ int) {}
