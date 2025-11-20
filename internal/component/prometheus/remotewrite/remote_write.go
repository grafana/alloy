package remotewrite

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/static/metrics/wal"
	"github.com/grafana/alloy/internal/useragent"
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

	walStore    *wal.Storage
	remoteStore *remote.Storage
	storage     storage.Storage
	exited      atomic.Bool

	mut sync.RWMutex
	cfg Arguments

	receiver *prometheus.Interceptor

	debugDataPublisher livedebugging.DebugDataPublisher
}

// New creates a new prometheus.remote_write component.
func New(o component.Options, args Arguments) (*Component, error) {
	// Older versions of prometheus.remote_write used the subpath below, which
	// added in too many extra unnecessary directories (since o.DataPath is
	// already unique).
	//
	// We best-effort attempt to delete the old path if it already exists to not
	// leak storage space.
	oldDataPath := filepath.Join(o.DataPath, "wal", o.ID)
	_ = os.RemoveAll(oldDataPath)

	walLogger := log.With(o.Logger, "subcomponent", "wal")
	walStorage, err := wal.NewStorage(walLogger, o.Registerer, o.DataPath)
	if err != nil {
		return nil, err
	}

	remoteLogger := slog.New(
		logging.NewSlogGoKitHandler(
			log.With(o.Logger, "subcomponent", "rw"),
		),
	)
	// TODO: Expose the option to enable type and unit labels: https://github.com/grafana/alloy/issues/4659
	remoteStore := remote.NewStorage(remoteLogger, o.Registerer, startTime, o.DataPath, remoteFlushDeadline, nil, false)

	walStorage.SetNotifier(remoteStore)

	service, err := o.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)

	if err := validateStabilityLevelForRemoteWritev2(o, args); err != nil {
		return nil, err
	}

	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	fanoutLogger := slog.New(
		logging.NewSlogGoKitHandler(
			log.With(o.Logger, "subcomponent", "fanout"),
		),
	)
	res := &Component{
		log:                o.Logger,
		opts:               o,
		walStore:           walStorage,
		remoteStore:        remoteStore,
		storage:            storage.NewFanout(fanoutLogger, walStorage, remoteStore),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	res.receiver = NewInterceptor(o.ID, &res.exited, res.debugDataPublisher, ls, res.storage)

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: res.receiver})

	if err := res.Update(args); err != nil {
		return nil, err
	}
	return res, nil
}

func startTime() (int64, error) { return 0, nil }

var _ component.Component = (*Component)(nil)
var _ component.LiveDebugging = (*Component)(nil)

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

	// Track the last timestamp we truncated for to prevent segments from getting
	// deleted until at least some new data has been sent.
	var lastTs = int64(math.MinInt64)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(c.truncateFrequency()):
			// We retrieve the current min/max keepalive time at once, since
			// retrieving them separately could lead to issues where we have an older
			// value for min which is now larger than max.
			c.mut.RLock()
			var (
				minWALTime = c.cfg.WALOptions.MinKeepaliveTime
				maxWALTime = c.cfg.WALOptions.MaxKeepaliveTime
			)
			c.mut.RUnlock()

			// The timestamp ts is used to determine which series are not receiving
			// samples and may be deleted from the WAL. Their most recent append
			// timestamp is compared to ts, and if that timestamp is older than ts,
			// they are considered inactive and may be deleted.
			//
			// Subtracting a duration from ts will delay when it will be considered
			// inactive and scheduled for deletion.
			ts := c.remoteStore.LowestSentTimestamp() - minWALTime.Milliseconds()
			if ts < 0 {
				ts = 0
			}

			// Network issues can prevent the result of LowestSentTimestamp from
			// changing. We don't want data in the WAL to grow forever, so we set a cap
			// on the maximum age data can be. If our ts is older than this cutoff point,
			// we'll shift it forward to start deleting very stale data.
			if maxTS := timestamp.FromTime(time.Now().Add(-maxWALTime)); ts < maxTS {
				ts = maxTS
			}

			if ts == lastTs {
				level.Debug(c.log).Log("msg", "not truncating the WAL, remote_write timestamp is unchanged", "ts", ts)
				continue
			}
			lastTs = ts

			level.Debug(c.log).Log("msg", "truncating the WAL", "ts", ts)
			err := c.walStore.Truncate(ts)
			if err != nil {
				// The only issue here is larger disk usage and a greater replay time,
				// so we'll only log this as a warning.
				level.Warn(c.log).Log("msg", "could not truncate WAL", "err", err)
			}
		}
	}
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

	convertedConfig, err := convertConfigs(cfg)
	if err != nil {
		return err
	}

	if err := validateStabilityLevelForRemoteWritev2(c.opts, cfg); err != nil {
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

func (c *Component) LiveDebugging() {}

func validateStabilityLevelForRemoteWritev2(o component.Options, args Arguments) error {
	for _, endpoint := range args.Endpoints {
		if endpoint.ProtobufMessage == PrometheusProtobufMessageV2 && !o.MinStability.Permits(featuregate.StabilityExperimental) {
			return fmt.Errorf("using remote write v2 (protobuf_message=%s) with endpoint %s requires setting the stability.level flag to experimental", PrometheusProtobufMessageV2, endpoint.Name)
		}
	}

	return nil
}
