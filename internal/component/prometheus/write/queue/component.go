package queue

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	promqueue "github.com/grafana/walqueue/implementations/prometheus"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.write.queue",
		Args:      Arguments{},
		Exports:   Exports{},
		Stability: featuregate.StabilityExperimental,
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewComponent(opts, args.(Arguments))
		},
	})
}

func NewComponent(opts component.Options, args Arguments) (*Queue, error) {
	level.Warn(opts.Logger).Log("msg", "prometheus.write.queue is deprecated and will be removed in a future version. Migrate to prometheus.remote_write to prevent future errors.")

	s := &Queue{
		opts:      opts,
		args:      args,
		log:       opts.Logger,
		endpoints: map[string]promqueue.Queue{},
	}
	s.opts.OnStateChange(Exports{Receiver: s})
	err := s.createEndpoints()
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Queue is a queue based WAL used to send data to a remote_write endpoint. Queue supports replaying
// and TTLs.
type Queue struct {
	mut       sync.RWMutex
	args      Arguments
	opts      component.Options
	log       log.Logger
	endpoints map[string]promqueue.Queue
	ctx       context.Context
}

// Run starts the component, blocking until ctx is canceled or the component
// suffers a fatal error. Run is guaranteed to be called exactly once per
// Component.
func (s *Queue) Run(ctx context.Context) error {
	s.ctx = ctx
	defer func() {
		s.mut.Lock()
		defer s.mut.Unlock()

		for _, ep := range s.endpoints {
			ep.Stop()
		}
	}()
	for _, ep := range s.endpoints {
		// If any of these fail to start thats a problem.
		err := ep.Start(ctx)
		if err != nil {
			return err
		}
	}
	<-ctx.Done()
	return nil
}

// Update provides a new Config to the component. The type of newConfig will
// always match the struct type which the component registers.
//
// Update will be called concurrently with Run. The component must be able to
// gracefully handle updating its config while still running.
//
// An error may be returned if the provided config is invalid.
func (s *Queue) Update(args component.Arguments) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	newArgs := args.(Arguments)
	// If they are the same do nothing.
	if reflect.DeepEqual(newArgs, s.args) {
		return nil
	}
	s.args = newArgs
	// Figure out which endpoint is new, which is updated, and which needs to be gone.
	// So add all the endpoints and then if they are in the new config then remove them from deletable.
	deletableEndpoints := make(map[string]struct{})
	for k := range s.endpoints {
		deletableEndpoints[k] = struct{}{}
	}

	for _, epCfg := range s.args.Endpoints {
		delete(deletableEndpoints, epCfg.Name)
		ep, found := s.endpoints[epCfg.Name]
		// If found stop and recreate.
		if found {
			// Stop and loose all the signals in the queue.
			// TODO drain the signals and re-add them
			ep.Stop()
		}
		nativeCfg := epCfg.ToNativeType()
		// Create
		end, err := promqueue.NewQueue(epCfg.Name, nativeCfg, filepath.Join(s.opts.DataPath, epCfg.Name, "wal"), uint32(s.args.Persistence.MaxSignalsToBatch), s.args.Persistence.BatchInterval, s.args.TTL, s.opts.Registerer, "alloy", s.opts.Logger)
		if err != nil {
			return err
		}
		err = end.Start(s.ctx)
		if err != nil {
			return err
		}
		s.endpoints[epCfg.Name] = end
	}
	// Now we need to figure out the endpoints that were not touched and able to be deleted.
	for name := range deletableEndpoints {
		s.endpoints[name].Stop()
		delete(s.endpoints, name)
	}
	return nil
}

func (s *Queue) createEndpoints() error {
	for _, ep := range s.args.Endpoints {
		nativeCfg := ep.ToNativeType()
		end, err := promqueue.NewQueue(ep.Name, nativeCfg, filepath.Join(s.opts.DataPath, ep.Name, "wal"), uint32(s.args.Persistence.MaxSignalsToBatch), s.args.Persistence.BatchInterval, s.args.TTL, s.opts.Registerer, "alloy", s.opts.Logger)
		if err != nil {
			return err
		}
		s.endpoints[ep.Name] = end
	}
	return nil
}

// Appender returns a new appender for the storage. The implementation
// can choose whether or not to use the context, for deadlines or to check
// for errors.
func (c *Queue) Appender(ctx context.Context) storage.Appender {
	c.mut.RLock()
	defer c.mut.RUnlock()

	children := make([]storage.Appender, 0)
	for _, ep := range c.endpoints {
		children = append(children, ep.Appender(ctx))
	}
	return &fanout{children: children}
}

func (c *Queue) String() string {
	return c.opts.ID + ".receiver"
}
