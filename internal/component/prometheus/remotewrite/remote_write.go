package remotewrite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/agent"
	"github.com/prometheus/prometheus/tsdb/wlog"
	"go.uber.org/atomic"
)

// Options.
//
// TODO(rfratto): This should be exposed. How do we want to expose this?
var remoteFlushDeadline = 1 * time.Minute

func init() {
	remote.UserAgent = useragent.Get()

	component.Register(component.Registration{
		Name:      "prometheus.remote_write",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			return New(o, c.(Arguments))
		},
	})
}

// Component is the prometheus.remote_write component.
type Component struct {
	log  log.Logger
	opts component.Options

	walStore    *agent.DB
	walOpts     *agent.Options
	remoteStore *remote.Storage
	storage     storage.Storage
	exited      atomic.Bool
	ls          labelstore.LabelStore

	mut sync.RWMutex
	cfg Arguments

	receiver *prometheus.Interceptor
}

// New creates a new prometheus.remote_write component.
func New(o component.Options, c Arguments) (*Component, error) {
	// Older versions of prometheus.remote_write used the subpath below, which
	// added in too many extra unnecessary directories (since o.DataPath is
	// already unique).
	//
	// We best-effort attempt to delete the old path if it already exists to not
	// leak storage space.
	oldDataPath := filepath.Join(o.DataPath, "wal", o.ID)
	_ = os.RemoveAll(oldDataPath)

	service, err := o.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)

	res := &Component{
		log:  o.Logger,
		opts: o,
		ls:   ls,
	}
	err = res.buildRemote(c)
	if err != nil {
		return nil, err
	}

	if err := res.Update(c); err != nil {
		return nil, err
	}
	return res, nil
}

func startTime() (int64, error) { return 0, nil }

var _ component.Component = (*Component)(nil)

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.exited.Store(true)

		level.Debug(c.log).Log("msg", "closing storage")
		err := c.storage.Close()
		level.Debug(c.log).Log("msg", "storage closed")
		if err != nil {
			level.Error(c.log).Log("msg", "error when closing storage", "err", err)
		}
	}()
	<-ctx.Done()
	return nil
}

func (c *Component) truncateFrequency() time.Duration {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.cfg.WALOptions.TruncateFrequency
}

// Update implements Component.
func (c *Component) Update(newConfig component.Arguments) error {
	cfg := newConfig.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	// Need to detect if the underlying agent.db information has changed.
	if c.walOpts.MinWALTime != cfg.WALOptions.MinKeepaliveTime.Milliseconds() || c.walOpts.MaxWALTime != cfg.WALOptions.MaxKeepaliveTime.Milliseconds() || c.walOpts.TruncateFrequency != cfg.WALOptions.TruncateFrequency {
		err := c.buildRemote(cfg)
		if err != nil {
			return err
		}
	}
	convertedConfig, err := convertConfigs(cfg)
	if err != nil {
		return err
	}
	uid := alloyseed.Get().UID
	for _, cfg := range convertedConfig.RemoteWriteConfigs {
		if cfg.Headers == nil {
			cfg.Headers = map[string]string{}
		}
		cfg.Headers[alloyseed.LegacyHeaderName] = uid
		cfg.Headers[alloyseed.HeaderName] = uid
	}

	err = c.remoteStore.ApplyConfig(convertedConfig)
	if err != nil {
		return err
	}
	c.cfg = cfg
	return nil
}

func (c *Component) buildRemote(args Arguments) error {
	if c.walStore != nil {
		// Lets do the hard thing of tearing it all down and rebuilding.
		err := c.storage.Close()
		level.Debug(c.log).Log("msg", "storage closed due to config change")
		if err != nil {
			level.Error(c.log).Log("msg", "error when closing storage when changing", "err", err)
			return err
		}
		err = c.remoteStore.Close()
		level.Debug(c.log).Log("msg", "remote store closed due to config change")
		if err != nil {
			level.Error(c.log).Log("msg", "error when closing remote when changing config", "err", err)
			return err
		}
		err = c.walStore.Close()
		level.Debug(c.log).Log("msg", "wal store closed due to config change")
		if err != nil {
			level.Error(c.log).Log("msg", "error when closing wal store when changing", "err", err)
			return err
		}
	}
	remoteLogger := log.With(c.opts.Logger, "subcomponent", "rw")
	remoteStore := remote.NewStorage(remoteLogger, c.opts.Registerer, startTime, c.opts.DataPath, remoteFlushDeadline, nil)

	walLogger := log.With(c.opts.Logger, "subcomponent", "wal")
	opts := agent.DefaultOptions()
	opts.MaxWALTime = args.WALOptions.MaxKeepaliveTime.Milliseconds()
	opts.MinWALTime = args.WALOptions.MinKeepaliveTime.Milliseconds()
	opts.TruncateFrequency = args.WALOptions.TruncateFrequency
	opts.WALCompression = wlog.CompressionSnappy
	walStorage, err := agent.Open(walLogger, c.opts.Registerer, remoteStore, c.opts.DataPath, opts)
	if err != nil {
		return err
	}
	walStorage.SetWriteNotified(remoteStore)
	c.walStore = walStorage
	c.walOpts = opts
	c.remoteStore = remoteStore
	c.storage = storage.NewFanout(c.opts.Logger, walStorage, remoteStore)
	c.receiver = prometheus.NewInterceptor(
		c.storage,
		c.ls,

		// In the methods below, conversion is needed because remote_writes assume
		// they are responsible for generating ref IDs. This means two
		// remote_writes may return the same ref ID for two different series. We
		// treat the remote_write ID as a "local ID" and translate it to a "global
		// ID" to ensure Alloy compatibility.

		prometheus.WithAppendHook(func(globalRef storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", c.opts.ID)
			}

			localID := c.ls.GetLocalRefID(c.opts.ID, uint64(globalRef))
			newRef, nextErr := next.Append(storage.SeriesRef(localID), l, t, v)
			if localID == 0 {
				c.ls.GetOrAddLink(c.opts.ID, uint64(newRef), l)
			}
			return globalRef, nextErr
		}),
		prometheus.WithHistogramHook(func(globalRef storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", c.opts.ID)
			}

			localID := c.ls.GetLocalRefID(c.opts.ID, uint64(globalRef))
			newRef, nextErr := next.AppendHistogram(storage.SeriesRef(localID), l, t, h, fh)
			if localID == 0 {
				c.ls.GetOrAddLink(c.opts.ID, uint64(newRef), l)
			}
			return globalRef, nextErr
		}),
		prometheus.WithMetadataHook(func(globalRef storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", c.opts.ID)
			}

			localID := c.ls.GetLocalRefID(c.opts.ID, uint64(globalRef))
			newRef, nextErr := next.UpdateMetadata(storage.SeriesRef(localID), l, m)
			if localID == 0 {
				c.ls.GetOrAddLink(c.opts.ID, uint64(newRef), l)
			}
			return globalRef, nextErr
		}),
		prometheus.WithExemplarHook(func(globalRef storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", c.opts.ID)
			}

			localID := c.ls.GetLocalRefID(c.opts.ID, uint64(globalRef))
			newRef, nextErr := next.AppendExemplar(storage.SeriesRef(localID), l, e)
			if localID == 0 {
				c.ls.GetOrAddLink(c.opts.ID, uint64(newRef), l)
			}
			return globalRef, nextErr
		}),
	)

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	c.opts.OnStateChange(Exports{Receiver: c.receiver})
	return nil
}
