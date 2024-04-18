package batch

import (
	"bytes"
	"context"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/storage"
)

// Create a byte sync.pool
var buf = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.remote_write",
		Args:      Arguments{},
		Exports:   Exports{},
		Stability: featuregate.StabilityExperimental,
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewComponent(opts, args.(Arguments))
		},
	})
}

func NewComponent(opts component.Options, args Arguments) (*Queue, error) {
	q, err := newFileQueue(filepath.Join(opts.DataPath, "wal"))
	if err != nil {
		return nil, err
	}
	s := &Queue{
		database: q,
		opts:     opts,
		b:        newBatch(q, args.BatchSize),
	}

	return s, s.Update(args)
}

// Queue is a queue based WAL used to send data to a remote_write endpoint. Queue supports replaying
// sending and TTLs.
type Queue struct {
	mut        sync.RWMutex
	database   *filequeue
	args       Arguments
	opts       component.Options
	wr         *writer
	testClient WriteClient
	b          *batch
}

// Run starts the component, blocking until ctx is canceled or the component
// suffers a fatal error. Run is guaranteed to be called exactly once per
// Component.
func (s *Queue) Run(ctx context.Context) error {
	go s.b.StartTimer(ctx)
	qms, err := s.newQueueManagers()
	if err != nil {
		return err
	}
	for _, qm := range qms {
		wr := newWriter(s.opts.ID, qm, s.database, s.opts.Logger)
		s.wr = wr
		started := make(chan struct{})
		go qm.Start(started)
		<-started
		go wr.Start(ctx)
	}
	<-ctx.Done()
	return nil
}

func (s *Queue) newQueueManagers() ([]*QueueManager, error) {
	qms := make([]*QueueManager, 0)
	for _, ed := range s.args.Endpoints {
		wr, err := s.newWriteClient(ed)
		if err != nil {
			return nil, err
		}
		qm := NewQueueManager(
			s.opts.Registerer,
			s.opts.Logger,
			ed.QueueOptions,
			ed.MetadataOptions,
			wr,
			1*time.Minute,
			&maxTimestamp{
				Gauge: prometheus.NewGauge(prometheus.GaugeOpts{
					Namespace: "prometheus",
					Subsystem: "remote_storage",
					Name:      "highest_timestamp_in_seconds",
					Help:      "Highest timestamp that has come into the remote storage via the Appender interface, in seconds since epoch.",
				}),
			},
			true,
			true,
		)
		qms = append(qms, qm)
	}
	return qms, nil
}
func (s *Queue) newWriteClient(ed *EndpointOptions) (WriteClient, error) {
	if s.testClient != nil {
		return s.testClient, nil
	}
	endUrl, err := url.Parse(ed.URL)
	if err != nil {
		return nil, err
	}
	cfgURL := &config_util.URL{URL: endUrl}
	if err != nil {
		return nil, err
	}

	wr, err := NewWriteClient(s.opts.ID, &ClientConfig{
		URL:              cfgURL,
		Timeout:          model.Duration(ed.RemoteTimeout),
		HTTPClientConfig: *ed.HTTPClientConfig.Convert(),
		SigV4Config:      nil,
		Headers:          ed.Headers,
		RetryOnRateLimit: ed.QueueOptions.RetryOnHTTP429,
	}, s.opts.Registerer)

	return wr, err
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

	s.args = args.(Arguments)
	s.opts.OnStateChange(Exports{Receiver: s})

	return nil
}

// Appender returns a new appender for the storage. The implementation
// can choose whether or not to use the context, for deadlines or to check
// for errors.
func (c *Queue) Appender(ctx context.Context) storage.Appender {
	c.mut.RLock()
	defer c.mut.RUnlock()

	return newAppender(c, c.args.TTL, c.b)
}

func (c *Queue) rollback(handles []string) {
	c.mut.Lock()
	defer c.mut.Unlock()
	for _, h := range handles {
		c.database.Delete(h)
	}
}

func (c *Queue) commit(handles []string) {
	c.mut.Lock()
	defer c.mut.Unlock()

	err := c.database.Commit(handles)
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to write commit", "err", err)
	}
}
