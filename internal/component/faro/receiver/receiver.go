package receiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-sourcemap/sourcemap"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "faro.receiver",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	log               log.Logger
	handler           *handler
	lazySourceMaps    *varSourceMapsStore
	sourceMapsMetrics *sourceMapMetrics
	serverMetrics     *serverMetrics

	argsMut sync.RWMutex
	args    Arguments

	metrics *metricsExporter
	logs    *logsExporter
	traces  *tracesExporter

	actorCh chan func(context.Context)

	healthMut sync.RWMutex
	health    component.Health
}

var _ component.HealthComponent = (*Component)(nil)

func New(o component.Options, args Arguments) (*Component, error) {
	var (
		// The source maps store changes at runtime based on settings, so we create
		// a lazy store to pass to the logs exporter.
		varStore = &varSourceMapsStore{}

		metrics = newMetricsExporter(o.Registerer)
		logs    = newLogsExporter(log.With(o.Logger, "exporter", "logs"), varStore, args.LogFormat)
		traces  = newTracesExporter(log.With(o.Logger, "exporter", "traces"))
	)

	c := &Component{
		log: o.Logger,
		handler: newHandler(
			log.With(o.Logger, "subcomponent", "handler"),
			o.Registerer,
			[]exporter{metrics, logs, traces},
		),
		lazySourceMaps:    varStore,
		sourceMapsMetrics: newSourceMapMetrics(o.Registerer),
		serverMetrics:     newServerMetrics(o.Registerer),

		metrics: metrics,
		logs:    logs,
		traces:  traces,

		actorCh: make(chan func(context.Context), 1),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	var (
		cancelCurrentActor context.CancelFunc
	)
	defer func() {
		if cancelCurrentActor != nil {
			cancelCurrentActor()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil

		case newActor := <-c.actorCh:
			// Terminate old actor (if any), and wait for it to return.
			if cancelCurrentActor != nil {
				cancelCurrentActor()
				wg.Wait()
			}

			// Run the new actor.
			actorCtx, actorCancel := context.WithCancel(ctx)
			cancelCurrentActor = actorCancel

			wg.Add(1)
			go func() {
				defer wg.Done()
				newActor(actorCtx)
			}()
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.argsMut.Lock()
	c.args = newArgs
	c.argsMut.Unlock()

	c.logs.SetLabels(newArgs.LogLabels)

	c.handler.Update(newArgs.Server)

	// Stop old store's cleanup if there is one
	c.lazySourceMaps.Stop()

	innerStore := newSourceMapsStore(
		log.With(c.log, "subcomponent", "handler"),
		newArgs.SourceMaps,
		c.sourceMapsMetrics,
		nil, // Use default HTTP client.
		nil, // Use default FS implementation.
	)
	c.lazySourceMaps.SetInner(innerStore)

	// Start cleanup for new store
	c.lazySourceMaps.Start()

	c.logs.SetReceivers(newArgs.Output.Logs)
	c.traces.SetConsumers(newArgs.Output.Traces)

	// Create a new server actor to run.
	makeNewServer := func(ctx context.Context) {
		// NOTE(rfratto): we don't use newArgs here, since it's not guaranteed that
		// our actor runs (we may be skipped for an existing scheduled function).
		// Instead, we load the most recent args.

		c.argsMut.RLock()
		var (
			args = c.args
		)
		c.argsMut.RUnlock()

		var cleanupWg sync.WaitGroup

		// Start cleanup routine if per-app rate limiting is enabled
		if c.handler.appRateLimiter != nil {
			cleanupWg.Go(func() {
				c.handler.appRateLimiter.CleanupRoutine(ctx, DEFAULT_CLEANUP_INTERVAL)
			})
		}

		srv := newServer(
			log.With(c.log, "subcomponent", "server"),
			args.Server,
			c.serverMetrics,
			c.handler,
		)

		// Reset health status.
		c.setServerHealth(nil)

		err := srv.Run(ctx)
		if err != nil {
			level.Error(c.log).Log("msg", "server exited with error", "err", err)
			c.setServerHealth(err)
		}

		cleanupWg.Wait()
	}

	select {
	case c.actorCh <- makeNewServer:
		// Actor has been scheduled to run.
	default:
		// An actor is already scheduled to run. Don't do anything.
	}

	return nil
}

func (c *Component) setServerHealth(err error) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()

	if err == nil {
		c.health = component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "component is ready to receive telemetry over the network",
			UpdateTime: time.Now(),
		}
	} else {
		c.health = component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    fmt.Sprintf("server has terminated: %s", err),
			UpdateTime: time.Now(),
		}
	}
}

// CurrentHealth implements component.HealthComponent. It returns an unhealthy
// status if the server has terminated.
func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

type varSourceMapsStore struct {
	mut   sync.RWMutex
	inner sourceMapsStore
}

var _ sourceMapsStore = (*varSourceMapsStore)(nil)

func (vs *varSourceMapsStore) GetSourceMap(sourceURL string, release string) (*sourcemap.Consumer, error) {
	vs.mut.RLock()
	defer vs.mut.RUnlock()

	if vs.inner != nil {
		return vs.inner.GetSourceMap(sourceURL, release)
	}

	return nil, fmt.Errorf("no sourcemap available")
}

func (vs *varSourceMapsStore) SetInner(inner sourceMapsStore) {
	vs.mut.Lock()
	defer vs.mut.Unlock()

	vs.inner = inner
}

func (vs *varSourceMapsStore) Start() {
	vs.mut.RLock()
	defer vs.mut.RUnlock()

	if vs.inner != nil {
		vs.inner.Start()
	}
}

func (vs *varSourceMapsStore) Stop() {
	vs.mut.RLock()
	defer vs.mut.RUnlock()

	if vs.inner != nil {
		vs.inner.Stop()
	}
}
