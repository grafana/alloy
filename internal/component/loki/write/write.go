package write

import (
	"context"
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
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

var (
	_ component.Component = (*Component)(nil)
)

// Component implements the loki.write component.
type Component struct {
	opts component.Options

	mut      sync.RWMutex
	args     Arguments
	receiver loki.LogsReceiver

	// remote write consumer
	consumer client.Consumer

	// sink is the place where log entries received by this component should be written to.
	// It will in turn write to client.Consumer.
	sink loki.EntryHandler
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
		// First we need to stop the sink. Stopping the sink will not stop the wrapped handler.
		if c.sink != nil {
			c.sink.Stop()
		}

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
		case entry := <-c.receiver.Chan():
			c.mut.RLock()
			select {
			case <-ctx.Done():
				c.mut.RUnlock()
				return nil
			case c.sink.Chan() <- entry:
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
	c.args = newArgs

	if c.sink != nil {
		c.sink.Stop()
	}

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

	c.sink = newEntryHandler(c.consumer, util.MapToModelLabelSet(c.args.ExternalLabels))

	return nil
}

func newEntryHandler(handler loki.EntryHandler, externalLabels model.LabelSet) loki.EntryHandler {
	return loki.NewEntryMutatorHandler(handler, func(e loki.Entry) loki.Entry {
		if len(externalLabels) == 0 {
			return e
		}
		e.Labels = externalLabels.Merge(e.Labels)
		return e
	})
}
