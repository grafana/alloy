package queue

import (
	"context"
	"reflect"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/write/queue/serialization"
	"github.com/grafana/alloy/internal/component/prometheus/write/queue/types"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/prometheus/storage"
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
	s := &Queue{
		opts:      opts,
		args:      args,
		log:       opts.Logger,
		endpoints: map[string]*endpoint{},
	}

	err := s.createEndpoints()
	if err != nil {
		return nil, err
	}
	// This needs to be started before we export the onstatechange so that it can accept
	// signals.
	for _, ep := range s.endpoints {
		ep.Start()
	}
	s.opts.OnStateChange(Exports{Receiver: s})

	return s, nil
}

// Queue is a queue based WAL used to send data to a remote_write endpoint. Queue supports replaying
// and TTLs.
type Queue struct {
	mut       sync.RWMutex
	args      Arguments
	opts      component.Options
	log       log.Logger
	endpoints map[string]*endpoint
}

// Run starts the component, blocking until ctx is canceled or the component
// suffers a fatal error. Run is guaranteed to be called exactly once per
// Component.
func (s *Queue) Run(ctx context.Context) error {
	defer func() {
		s.mut.Lock()
		defer s.mut.Unlock()

		for _, ep := range s.endpoints {
			ep.Stop()
		}
	}()

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
	sync.OnceFunc(func() {
		s.opts.OnStateChange(Exports{Receiver: s})
	})
	// If they are the same do nothing.
	if reflect.DeepEqual(newArgs, s.args) {
		return nil
	}
	s.args = newArgs
	// Figure out which endpoint is new, which is updated, and which needs to be gone.
	deletableEndpoints := make(map[string]struct{})
	for k := range s.endpoints {
		deletableEndpoints[k] = struct{}{}
	}

	for _, epCfg := range s.args.Endpoints {
		delete(deletableEndpoints, epCfg.Name)
		ep, ok := s.endpoints[epCfg.Name]
		if ok {
			// Update
			err := ep.Network().UpdateConfig(context.Background(), epCfg.ToNativeType())
			if err != nil {
				return err
			}
			err = ep.Serializer().UpdateConfig(context.Background(), types.SerializerConfig{
				MaxSignalsInBatch: uint32(s.args.Persistence.MaxSignalsToBatch),
				FlushFrequency:    s.args.Persistence.BatchInterval,
			})
			if err != nil {
				return err
			}
		} else {
			// Create
			end, err := createEndpoint(epCfg, s.args.TTL, uint(s.args.Persistence.MaxSignalsToBatch), s.args.Persistence.BatchInterval, s.opts.DataPath, s.opts.Registerer, s.opts.Logger)
			if err != nil {
				return err
			}
			end.Start()
			s.endpoints[epCfg.Name] = end
		}
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
		end, err := createEndpoint(ep, s.args.TTL, uint(s.args.Persistence.MaxSignalsToBatch), s.args.TTL, s.opts.DataPath, s.opts.Registerer, s.opts.Logger)
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
		children = append(children, serialization.NewAppender(ctx, c.args.TTL, ep.serializer, c.opts.Logger))
	}
	return &fanout{children: children}
}
