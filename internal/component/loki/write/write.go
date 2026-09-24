package write

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/prometheus/common/model"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.write",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Exports holds the receiver that is used to send log entries to the
// loki.write component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

var (
	_ component.Component = (*Component)(nil)
)

// Component implements the loki.write component.
type Component struct {
	opts     component.Options
	receiver loki.LogsReceiver

	mut            sync.RWMutex
	externalLabels model.LabelSet

	// remote write consumer
	consumer client.Consumer
}

// New creates a new loki.write component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts: o,
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	c.receiver = loki.NewLogsReceiver(loki.WithComponentID(o.ID))
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
		c.mut.Lock()
		defer c.mut.Unlock()

		if c.consumer != nil {
			if d, ok := c.consumer.(client.DrainableConsumer); ok {
				// stop and drain since component is shutting down.
				d.StopAndDrain()
			} else {
				c.consumer.Stop()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-c.receiver.Chan():
			c.consumeEntry(ctx, e)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	if err := validateConfigStabilityLevel(c.opts, newArgs); err != nil {
		return err
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	if c.consumer != nil {
		// only drain on component shutdown
		c.consumer.Stop()
	}

	c.externalLabels = util.MapToModelLabelSet(newArgs.ExternalLabels)

	cfgs := newArgs.convertEndpointConfigs()

	uid := alloyseed.Get().UID
	for i := range cfgs {
		//cfgs is slice of struct values, so we set by index
		if cfgs[i].Headers == nil {
			cfgs[i].Headers = map[string]string{}
		}
		cfgs[i].Headers[alloyseed.LegacyHeaderName] = uid
		cfgs[i].Headers[alloyseed.HeaderName] = uid
	}
	walCfg := wal.Config{
		Dir:           filepath.Join(c.opts.DataPath, "wal"),
		MaxSegmentAge: newArgs.WAL.MaxSegmentAge,
		WatchConfig: wal.WatchConfig{
			MinReadFrequency: newArgs.WAL.MinReadFrequency,
			MaxReadFrequency: newArgs.WAL.MaxReadFrequency,
			DrainTimeout:     newArgs.WAL.DrainTimeout,
		},
	}

	var err error
	if newArgs.WAL.Enabled {
		c.consumer, err = client.NewWALConsumer(c.opts.Logger, c.opts.Registerer, walCfg, cfgs...)
	} else {
		c.consumer, err = client.NewFanoutConsumer(c.opts.Logger, c.opts.Registerer, cfgs...)
	}

	if err != nil {
		return fmt.Errorf("failed to create clients: %w", err)
	}

	return nil
}

func (c *Component) consumeEntry(ctx context.Context, e loki.Entry) {
	c.mut.RLock()
	consumer := c.consumer

	if len(c.externalLabels) > 0 {
		e.Labels = c.externalLabels.Merge(e.Labels)
	}
	c.mut.RUnlock()

	// NOTE: For now it's ok to ignore error here. Error mean the consumer is going away,
	// either because ctx was canceled or because it has been stopped by a shutdown or an update.
	_ = consumer.ConsumeEntry(ctx, e)
}

func validateConfigStabilityLevel(o component.Options, args Arguments) error {
	canUseExperimentalConfig := o.MinStability.Permits(featuregate.StabilityExperimental)
	for _, e := range args.Endpoints {
		if e.QueueConfig != defaultQueueConfigArguments && !canUseExperimentalConfig {
			return errors.New("changing queue_config requires stability.level flag to be experimental")
		}
	}
	return nil
}
