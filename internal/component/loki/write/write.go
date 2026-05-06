package write

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

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

// Arguments holds values which are used to configure the loki.write component.
type Arguments struct {
	Endpoints      []EndpointOptions `alloy:"endpoint,block,optional"`
	ExternalLabels map[string]string `alloy:"external_labels,attr,optional"`
	MaxStreams     int               `alloy:"max_streams,attr,optional"`
	WAL            WalArguments      `alloy:"wal,block,optional"`
}

// WalArguments holds the settings for configuring the Write-Ahead Log (WAL) used
// by the underlying remote write client.
type WalArguments struct {
	Enabled          bool          `alloy:"enabled,attr,optional"`
	MaxSegmentAge    time.Duration `alloy:"max_segment_age,attr,optional"`
	MinReadFrequency time.Duration `alloy:"min_read_frequency,attr,optional"`
	MaxReadFrequency time.Duration `alloy:"max_read_frequency,attr,optional"`
	DrainTimeout     time.Duration `alloy:"drain_timeout,attr,optional"`
}

func (wa *WalArguments) Validate() error {
	if wa.MinReadFrequency >= wa.MaxReadFrequency {
		return fmt.Errorf("WAL min read frequency should be lower than max read frequency")
	}
	return nil
}

func (wa *WalArguments) SetToDefault() {
	// todo(thepalbi): Once we are in a good state: replay implemented, and a better cleanup mechanism
	// make WAL enabled the default
	*wa = WalArguments{
		Enabled:          false,
		MaxSegmentAge:    wal.DefaultMaxSegmentAge,
		MinReadFrequency: wal.DefaultWatchConfig.MinReadFrequency,
		MaxReadFrequency: wal.DefaultWatchConfig.MaxReadFrequency,
		DrainTimeout:     wal.DefaultWatchConfig.DrainTimeout,
	}
}

// Exports holds the receiver that is used to send log entries to the
// loki.write component.
type Exports struct {
	Receiver loki.Consumer `alloy:"receiver,attr"`
}

var (
	_ component.Component = (*Component)(nil)
	_ loki.Consumer       = (*Component)(nil)
)

// Component implements the loki.write component.
type Component struct {
	opts component.Options

	mut     sync.RWMutex
	stopped bool
	args    Arguments
	labels  model.LabelSet

	// remote write consumer
	consumer client.Consumer
}

// New creates a new loki.write component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts: o,
	}

	o.OnStateChange(Exports{Receiver: c})

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
		c.stopped = true

		if c.consumer != nil {
			if d, ok := c.consumer.(client.DrainableConsumer); ok {
				// stop and drain since component is shutting down.
				d.StopAndDrain()
			} else {
				c.consumer.Stop()
			}
		}
	}()

	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	if err := validateConfigStabilityLevel(c.opts, newArgs); err != nil {
		return err
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = newArgs
	c.labels = util.MapToModelLabelSet(c.args.ExternalLabels)

	if c.consumer != nil {
		// only drain on component shutdown
		c.consumer.Stop()
	}

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
		Enabled:       newArgs.WAL.Enabled,
		Dir:           filepath.Join(c.opts.DataPath, "wal"),
		MaxSegmentAge: newArgs.WAL.MaxSegmentAge,
		WatchConfig: wal.WatchConfig{
			MinReadFrequency: newArgs.WAL.MinReadFrequency,
			MaxReadFrequency: newArgs.WAL.MaxReadFrequency,
			DrainTimeout:     newArgs.WAL.DrainTimeout,
		},
	}

	var err error
	if walCfg.Enabled {
		c.consumer, err = client.NewWALConsumer(c.opts.Logger, c.opts.Registerer, walCfg, cfgs...)
	} else {
		c.consumer, err = client.NewFanoutConsumer(c.opts.Logger, c.opts.Registerer, cfgs...)
	}

	if err != nil {
		return fmt.Errorf("failed to create cliens: %w", err)
	}

	return nil
}

func (c *Component) Consume(ctx context.Context, batch loki.Batch) error {
	return errors.New("unimplemented")
}

func (c *Component) ConsumeEntry(ctx context.Context, entry loki.Entry) error {
	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.stopped {
		return loki.ErrConsumerStopped
	}

	if len(c.labels) != 0 {
		entry.Labels = c.labels.Merge(entry.Labels)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.consumer.Chan() <- entry:
		return nil
	}
}

func (c *Component) String() string {
	return c.opts.ID + ".receiver"
}

func validateConfigStabilityLevel(o component.Options, args Arguments) error {
	canUseExperimentalConfig := o.MinStability.Permits(featuregate.StabilityExperimental)
	for _, e := range args.Endpoints {
		if e.QueueConfig != defaultQueueConfig && !canUseExperimentalConfig {
			return errors.New("changing queue_config requires stability.level flag to be experimental")
		}
	}
	return nil
}
