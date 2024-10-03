package queue

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/filequeue"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/network"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/serialization"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/prometheus/storage"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.remote.queue",
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
	// TODO @mattdurham need to cycle through the endpoints figuring out what changed instead of this global stop and start..
	if len(s.endpoints) > 0 {
		for _, ep := range s.endpoints {
			ep.Stop()
		}
		s.endpoints = map[string]*endpoint{}
	}
	return s.createEndpoints()
}

func (s *Queue) createEndpoints() error {
	for _, ep := range s.args.Connections {
		reg := prometheus.WrapRegistererWith(prometheus.Labels{"endpoint": ep.Name}, s.opts.Registerer)
		stats := types.NewStats("alloy", "queue_series", reg)
		stats.BackwardsCompatibility(reg)
		meta := types.NewStats("alloy", "queue_metadata", reg)
		cfg := types.ConnectionConfig{
			URL:        ep.URL,
			BatchCount: ep.BatchCount,
			// Functionally this cannot go below 1s
			FlushFrequency: ep.FlushFrequency,
			Timeout:        ep.Timeout,
			UserAgent:      "alloy",
			ExternalLabels: s.args.ExternalLabels,
			Connections:    ep.Connections,
		}
		if ep.BasicAuth != nil {
			cfg.BasicAuth = &types.BasicAuth{
				Username: ep.BasicAuth.Username,
				Password: string(ep.BasicAuth.Password),
			}
		}
		client, err := network.New(cfg, s.log, stats.UpdateNetwork, meta.UpdateNetwork)

		if err != nil {
			return err
		}
		// Serializer is set after
		end := NewEndpoint(client, nil, stats, meta, s.args.TTL, s.opts.Logger)
		// This wait group is to ensure we are started before we send on the mailbox.
		fq, err := filequeue.NewQueue(filepath.Join(s.opts.DataPath, ep.Name, "wal"), func(ctx context.Context, dh types.DataHandle) {
			_ = end.incoming.Send(ctx, dh)
		}, s.opts.Logger)
		if err != nil {
			return err
		}
		serial, err := serialization.NewSerializer(types.SerializerConfig{
			MaxSignalsInBatch: 10_000,
			FlushFrequency:    1 * time.Second,
		}, fq, stats.UpdateFileQueue, s.opts.Logger)
		if err != nil {
			return err
		}
		end.serializer = serial
		s.endpoints[ep.Name] = end
		// endpoint is responsible for starting all the children, this way they spin up
		// together and are town down together. Or at least handled internally.
		end.Start()
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
