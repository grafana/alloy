// Package consumer implements the kafka.tenant_consumer component, which
// consumes records written by kafka.tenant_producer and feeds them into Alloy
// pipelines. Each signal has its own topic, and each tenant owns the same
// partition number in every topic. All consumers join one group, so each
// tenant's signal is processed by exactly one consumer at a time, and each
// partition is processed by its own worker so a stalled partition doesn't
// hold up any other.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/prometheus/storage"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
	"github.com/grafana/alloy/internal/component/kafka/tenant/registry"
	lokisource "github.com/grafana/alloy/internal/component/loki/source"
	lokiapi "github.com/grafana/alloy/internal/component/loki/source/api"
	"github.com/grafana/alloy/internal/component/otelcol"
	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "kafka.tenant_consumer",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures kafka.tenant_consumer.
type Arguments struct {
	Registry       alloytypes.OptionalSecret `alloy:"registry,attr"`
	GroupID        string                    `alloy:"group_id,attr,optional"`
	InstanceID     string                    `alloy:"instance_id,attr,optional"`
	StartOffset    string                    `alloy:"start_offset,attr,optional"`
	SessionTimeout time.Duration             `alloy:"session_timeout,attr,optional"`

	MaxRetries int           `alloy:"max_retries,attr,optional"`
	MinBackoff time.Duration `alloy:"min_backoff,attr,optional"`
	MaxBackoff time.Duration `alloy:"max_backoff,attr,optional"`

	Client kafkaclient.Arguments `alloy:"client,block"`

	MetricsForwardTo  []storage.Appendable       `alloy:"metrics_forward_to,attr,optional"`
	LogsForwardTo     []loki.LogsReceiver        `alloy:"logs_forward_to,attr,optional"`
	ProfilesForwardTo []pyroscope.Appendable     `alloy:"profiles_forward_to,attr,optional"`
	Output            *otelcol.ConsumerArguments `alloy:"output,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		GroupID:        "alloy-consumers",
		StartOffset:    "earliest",
		SessionTimeout: 45 * time.Second,
		MaxRetries:     5,
		MinBackoff:     100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Output:         &otelcol.ConsumerArguments{},
	}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.GroupID == "" {
		return errors.New("group_id must not be empty")
	}
	if a.StartOffset != "earliest" && a.StartOffset != "latest" {
		return fmt.Errorf("start_offset must be \"earliest\" or \"latest\", got %q", a.StartOffset)
	}
	if a.MaxRetries < 0 {
		return errors.New("max_retries must not be negative")
	}
	return nil
}

// groupArgs is the subset of Arguments that requires recreating the Kafka
// client when changed.
type groupArgs struct {
	Client         kafkaclient.Arguments
	GroupID        string
	InstanceID     string
	StartOffset    string
	SessionTimeout time.Duration
	Topics         []string
}

func (a Arguments) groupArgs(topics []string) groupArgs {
	return groupArgs{
		Client:         a.Client,
		GroupID:        a.GroupID,
		InstanceID:     a.InstanceID,
		StartOffset:    a.StartOffset,
		SessionTimeout: a.SessionTimeout,
		Topics:         topics,
	}
}

// Component implements kafka.tenant_consumer.
type Component struct {
	opts    component.Options
	metrics *metrics

	registry registry.Holder
	restart  chan struct{}

	promFanout  *alloyprom.Fanout
	promHandler http.Handler
	lokiFanout  *loki.Fanout
	lokiRoute   lokisource.LogsRoute
	pyroFanout  *pyroscope.Fanout

	mut       sync.Mutex
	args      Arguments
	groupArgs groupArgs
	health    component.Health

	// processHook, if set, is called before and after a worker processes a
	// batch of records. Used by tests.
	processHook func(topic string, partition int32, start bool)
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new kafka.tenant_consumer component.
func New(opts component.Options, args Arguments) (*Component, error) {
	service, err := opts.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)
	promFanout := alloyprom.NewFanout(args.MetricsForwardTo, opts.ID, opts.Registerer, ls)

	c := &Component{
		opts:    opts,
		metrics: newMetrics(opts.Registerer),
		restart: make(chan struct{}, 1),

		promFanout: promFanout,
		promHandler: promremote.NewWriteHandler(
			opts.Logger,
			opts.Registerer,
			promFanout,
			remote.MessageTypes{remote.WriteV1MessageType},
			false, // ingestCTZeroSample
			false, // enableTypeAndUnitLabels
			true,  // appendMetadata
		),
		lokiFanout: loki.NewFanout(args.LogsForwardTo),
		lokiRoute:  lokiapi.NewLokiPushRoute(0),
		pyroFanout: pyroscope.NewFanout(args.ProfilesForwardTo, opts.ID, opts.Registerer),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.promFanout.Clear()
	for {
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			c.runConsumer(runCtx)
		}()

		select {
		case <-ctx.Done():
			cancel()
			<-done
			return nil
		case <-c.restart:
			c.opts.Logger.Info("kafka client settings changed, restarting consumer")
			cancel()
			<-done
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// Reject a bad registry as a whole; the previous one stays in use.
	reg, err := registry.Parse([]byte(newArgs.Registry.Value))
	if err != nil {
		return err
	}

	c.promFanout.UpdateChildren(newArgs.MetricsForwardTo)
	c.lokiFanout.UpdateChildren(newArgs.LogsForwardTo)
	c.pyroFanout.UpdateChildren(newArgs.ProfilesForwardTo)
	c.registry.Store(reg)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs
	if ga := newArgs.groupArgs(reg.Topics()); !reflect.DeepEqual(c.groupArgs, ga) {
		c.groupArgs = ga
		select {
		case c.restart <- struct{}{}:
		default:
		}
	}
	return nil
}

func (c *Component) currentArgs() Arguments {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.args
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.health
}

func (c *Component) setHealth(typ component.HealthType, msg string) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.health = component.Health{Health: typ, Message: msg, UpdateTime: time.Now()}
}

// runConsumer runs one consumer group session until ctx is canceled.
func (c *Component) runConsumer(ctx context.Context) {
	// Drain a pending restart: this session already uses the latest args.
	select {
	case <-c.restart:
	default:
	}
	args := c.currentArgs()

	if !c.waitForTopics(ctx, args) {
		return
	}

	s := newSession(ctx, c)
	cl, err := c.newGroupClient(args, s)
	if err != nil {
		c.setHealth(component.HealthTypeUnhealthy, err.Error())
		c.opts.Logger.Error("failed to create kafka consumer", "err", err)
		<-ctx.Done()
		return
	}
	defer cl.Close()

	// A previous session may not have seen its partitions revoked.
	c.metrics.assignedPartitions.Set(0)
	c.metrics.lag.Reset()
	c.setHealth(component.HealthTypeHealthy, "consuming")

	s.poll(cl)
	s.shutdown(cl)
}

// waitForTopics blocks until the registry's topics have been verified. It
// returns false if ctx was canceled first.
func (c *Component) waitForTopics(ctx context.Context, args Arguments) bool {
	for {
		err := c.checkTopics(ctx, args)
		if err == nil {
			return true
		} else if ctx.Err() != nil {
			return false
		}
		c.setHealth(component.HealthTypeUnhealthy, err.Error())
		c.opts.Logger.Warn("topic check failed, not consuming", "err", err)

		select {
		case <-ctx.Done():
			return false
		case <-time.After(10 * time.Second):
		}
	}
}

func (c *Component) checkTopics(ctx context.Context, args Arguments) error {
	opts, err := args.Client.Opts()
	if err != nil {
		return err
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return kafkaclient.CheckTopics(ctx, cl, c.registry.Load())
}

func (c *Component) newGroupClient(args Arguments, s *session) (*kgo.Client, error) {
	opts, err := args.Client.Opts()
	if err != nil {
		return nil, err
	}

	resetOffset := kgo.NewOffset().AtStart()
	if args.StartOffset == "latest" {
		resetOffset = kgo.NewOffset().AtEnd()
	}

	opts = append(opts,
		kgo.ConsumerGroup(args.GroupID),
		kgo.ConsumeTopics(c.registry.Load().Topics()...),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
		// Only offsets of records a worker finished are committed.
		kgo.AutoCommitMarks(),
		kgo.AutoCommitInterval(time.Second),
		// Rebalance callbacks never run while a poll is being dispatched,
		// so workers can be stopped before their partitions are given away.
		kgo.BlockRebalanceOnPoll(),
		kgo.ConsumeResetOffset(resetOffset),
		kgo.SessionTimeout(args.SessionTimeout),
		kgo.OnPartitionsAssigned(s.onAssigned),
		kgo.OnPartitionsRevoked(s.onRevoked),
		kgo.OnPartitionsLost(s.onLost),
	)
	if args.InstanceID != "" {
		// Static membership: a restart within the session timeout gets its
		// partitions back without a rebalance.
		opts = append(opts, kgo.InstanceID(args.InstanceID))
	}
	return kgo.NewClient(opts...)
}
