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

	// wal is opened when it is first enabled and reused across updates.
	// It will always be nil when disabled.
	wal wal.WAL
	// consumer is set by the first successful Update and afterwards only replaced by
	// another successfully built consumer, never cleared.
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

		if d, ok := c.consumer.(client.DrainableConsumer); ok {
			// stop and drain since component is shutting down.
			d.StopAndDrain()
		} else {
			c.consumer.Stop()
		}

		if c.wal != nil {
			c.wal.Close()
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

	var (
		consumer client.Consumer
		err      error
	)

	if newArgs.WAL.Enabled {
		consumer, err = c.newWALConsumer(newArgs, cfgs)
	} else {
		consumer, err = client.NewFanoutConsumer(c.opts.Logger, c.opts.Registerer, cfgs...)
	}

	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if c.consumer != nil {
		c.consumer.Stop()
	}

	// The WAL is only needed while a WAL consumer is reading from it.
	if !newArgs.WAL.Enabled && c.wal != nil {
		c.wal.Close()
		c.wal = nil
	}

	c.consumer = consumer
	c.consumer.Start()
	c.externalLabels = util.MapToModelLabelSet(newArgs.ExternalLabels)

	return nil
}

func (c *Component) newWALConsumer(args Arguments, cfgs []client.Config) (client.Consumer, error) {
	var (
		wl     = c.wal
		opened = false
	)

	if wl == nil {
		var err error
		wl, err = wal.New(c.opts.Logger, c.opts.Registerer, filepath.Join(c.opts.DataPath, "wal"))
		if err != nil {
			return nil, err
		}
		opened = true
	}

	consumer, err := client.NewWALConsumer(
		c.opts.Logger,
		c.opts.Registerer,
		wl,
		wal.Config{
			MaxSegmentAge: args.WAL.MaxSegmentAge,
			WatchConfig: wal.WatchConfig{
				MinReadFrequency: args.WAL.MinReadFrequency,
				MaxReadFrequency: args.WAL.MaxReadFrequency,
				DrainTimeout:     args.WAL.DrainTimeout,
			},
		},
		cfgs...,
	)
	if err != nil {
		if opened {
			wl.Close()
		}
		return nil, err
	}

	c.wal = wl
	return consumer, nil
}

func (c *Component) consumeEntry(ctx context.Context, e loki.Entry) {
	var labelsMerged bool

	for {
		c.mut.RLock()
		var (
			consumer       = c.consumer
			externalLabels = c.externalLabels
		)
		c.mut.RUnlock()

		if len(externalLabels) > 0 && !labelsMerged {
			labelsMerged = true
			e.Labels = externalLabels.Merge(e.Labels)
		}

		err := consumer.ConsumeEntry(ctx, e)
		// Only a stopped consumer is worth retrying, an update swapped it out.
		// Anything else is an accepted entry, a canceled context while shutting down,
		// or a failed WAL write.
		// This makes delivery at-least-once: a fanout consumer enqueues to its
		// endpoints in order, so it can be stopped after some of them already
		// accepted the entry, and the retry hands it to those endpoints again.
		if !errors.Is(err, loki.ErrConsumerStopped) {
			return
		}

		if ctx.Err() != nil {
			return
		}
	}
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
