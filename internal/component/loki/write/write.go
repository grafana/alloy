package write

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/grafana/agent/internal/agentseed"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/agent/internal/component/common/loki/client"
	"github.com/grafana/agent/internal/component/common/loki/limit"
	"github.com/grafana/agent/internal/component/common/loki/wal"
	"github.com/grafana/agent/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.write",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.write component.
type Arguments struct {
	Endpoints      []EndpointOptions `river:"endpoint,block,optional"`
	ExternalLabels map[string]string `river:"external_labels,attr,optional"`
	MaxStreams     int               `river:"max_streams,attr,optional"`
	WAL            WalArguments      `river:"wal,block,optional"`
}

// WalArguments holds the settings for configuring the Write-Ahead Log (WAL) used
// by the underlying remote write client.
type WalArguments struct {
	Enabled          bool          `river:"enabled,attr,optional"`
	MaxSegmentAge    time.Duration `river:"max_segment_age,attr,optional"`
	MinReadFrequency time.Duration `river:"min_read_frequency,attr,optional"`
	MaxReadFrequency time.Duration `river:"max_read_frequency,attr,optional"`
	DrainTimeout     time.Duration `river:"drain_timeout,attr,optional"`
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
	Receiver loki.LogsReceiver `river:"receiver,attr"`
}

var (
	_ component.Component = (*Component)(nil)
)

// Component implements the loki.write component.
type Component struct {
	opts    component.Options
	metrics *client.Metrics

	mut      sync.RWMutex
	args     Arguments
	receiver loki.LogsReceiver

	// remote write components
	clientManger *client.Manager
	walWriter    *wal.Writer

	// sink is the place where log entries received by this component should be written to. If WAL
	// is enabled, this will be the WAL Writer, otherwise, the client manager
	sink loki.EntryHandler
}

// New creates a new loki.write component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:    o,
		metrics: client.NewMetrics(o.Registerer),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	c.receiver = loki.NewLogsReceiver()
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
		// when exiting Run, proceed to shut down first the writer component, and then
		// the client manager, with the WAL and remote-write client inside
		if c.walWriter != nil {
			c.walWriter.Stop()
		}
		if c.clientManger != nil {
			// drain, since the component is shutting down. That means the agent is shutting down as well
			c.clientManger.StopWithDrain(true)
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

	if c.walWriter != nil {
		c.walWriter.Stop()
	}
	if c.clientManger != nil {
		// only drain on component shutdown
		c.clientManger.Stop()
	}

	cfgs := newArgs.convertClientConfigs()

	uid := agentseed.Get().UID
	for i := range cfgs {
		//cfgs is slice of struct values, so we set by index
		if cfgs[i].Headers == nil {
			cfgs[i].Headers = map[string]string{}
		}
		cfgs[i].Headers[agentseed.HeaderName] = uid
	}
	walCfg := wal.Config{
		Enabled:       newArgs.WAL.Enabled,
		MaxSegmentAge: newArgs.WAL.MaxSegmentAge,
		WatchConfig: wal.WatchConfig{
			MinReadFrequency: newArgs.WAL.MinReadFrequency,
			MaxReadFrequency: newArgs.WAL.MaxReadFrequency,
			DrainTimeout:     newArgs.WAL.DrainTimeout,
		},
	}

	// Update WAL dir with DataPath subdir
	walCfg.Dir = filepath.Join(c.opts.DataPath, "wal")

	var err error
	var notifier client.WriterEventsNotifier = client.NilNotifier
	// nil-out wal writer in case WAL was disabled
	c.walWriter = nil
	// only configure WAL Writer if enabled
	if walCfg.Enabled {
		c.walWriter, err = wal.NewWriter(walCfg, c.opts.Logger, c.opts.Registerer)
		if err != nil {
			return fmt.Errorf("error creating wal writer: %w", err)
		}
		notifier = c.walWriter
	}

	c.clientManger, err = client.NewManager(c.metrics, c.opts.Logger, limit.Config{
		MaxStreams: newArgs.MaxStreams,
	}, c.opts.Registerer, walCfg, notifier, cfgs...)
	if err != nil {
		return fmt.Errorf("failed to create client manager: %w", err)
	}

	// if WAL is enabled, the WAL writer should be the destination sink. Otherwise, the client manager
	if walCfg.Enabled {
		c.sink = c.walWriter
	} else {
		c.sink = c.clientManger
	}

	return err
}
