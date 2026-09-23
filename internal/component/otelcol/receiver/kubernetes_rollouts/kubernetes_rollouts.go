// Package kubernetes_rollouts provides the otelcol.receiver.kubernetes_rollouts component.
package kubernetes_rollouts

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/grafana/ckit/shard"
	"go.opentelemetry.io/collector/consumer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/grafana/alloy/internal/component"
	commonk8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.kubernetes_rollouts",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures otelcol.receiver.kubernetes_rollouts.
type Arguments struct {
	ClusterName string `alloy:"cluster_name,attr,optional"`
	ClusterUID  string `alloy:"cluster_uid,attr,optional"`

	Client     commonk8s.ClientArguments  `alloy:"client,block,optional"`
	Clustering cluster.ComponentBlock     `alloy:"clustering,block,optional"`
	Output     *otelcol.ConsumerArguments `alloy:"output,block"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{Client: commonk8s.DefaultClientArguments}
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Output == nil {
		return fmt.Errorf("output block is required")
	}
	return nil
}

// Component watches Kubernetes Deployment rollouts and emits OpenTelemetry events.
type Component struct {
	opts    component.Options
	cluster cluster.Cluster

	mu         sync.RWMutex
	args       Arguments
	restConfig *rest.Config
	logsSink   consumer.Logs
	restart    chan struct{}
}

var (
	_ component.Component = (*Component)(nil)
	_ cluster.Component   = (*Component)(nil)
)

// New creates a new otelcol.receiver.kubernetes_rollouts component.
func New(opts component.Options, args Arguments) (*Component, error) {
	clusterData, err := opts.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("getting cluster service: %w", err)
	}

	c := &Component{
		opts:    opts,
		cluster: clusterData.(cluster.Cluster),
		restart: make(chan struct{}, 1),
	}
	if err := c.update(args, false); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	for {
		generationCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- c.runGeneration(generationCtx) }()

		select {
		case <-ctx.Done():
			cancel()
			<-done
			return nil
		case <-c.restart:
			cancel()
			<-done
			continue
		case err := <-done:
			cancel()
			if err == nil || ctx.Err() != nil {
				return nil
			}
			c.opts.Logger.Error("Kubernetes rollout watcher stopped; retrying", "err", err)
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-c.restart:
				timer.Stop()
			case <-timer.C:
			}
		}
	}
}

func (c *Component) runGeneration(ctx context.Context) error {
	c.mu.RLock()
	args := c.args
	restConfig := rest.CopyConfig(c.restConfig)
	c.mu.RUnlock()

	if args.Clustering.Enabled {
		if !c.cluster.Ready() {
			<-ctx.Done()
			return nil
		}
		owners, err := c.cluster.Lookup(shard.StringKey(c.opts.ID), 1, shard.OpReadWrite)
		if err != nil {
			return fmt.Errorf("determining rollout watcher ownership: %w", err)
		}
		if len(owners) != 1 || !owners[0].Self {
			<-ctx.Done()
			return nil
		}
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}
	ctrl := newController(controllerOptions{
		logger:      c.opts.Logger,
		client:      client,
		clusterName: args.ClusterName,
		clusterUID:  args.ClusterUID,
		emit:        c.emit,
	})
	return ctrl.run(ctx)
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	return c.update(args.(Arguments), true)
}

func (c *Component) update(args Arguments, signal bool) error {
	if args.Output == nil {
		return fmt.Errorf("output block is required")
	}
	restConfig, err := args.Client.BuildRESTConfig(c.opts.Logger)
	if err != nil {
		return fmt.Errorf("building Kubernetes client configuration: %w", err)
	}

	c.mu.Lock()
	changed := !reflect.DeepEqual(c.args, args)
	c.args = args
	c.restConfig = restConfig
	c.logsSink = fanoutconsumer.Logs(args.Output.Logs)
	c.mu.Unlock()

	if signal && changed {
		c.requestRestart()
	}
	return nil
}

func (c *Component) emit(ctx context.Context, batch func() eventBatch) error {
	c.mu.RLock()
	sink := c.logsSink
	c.mu.RUnlock()
	return sink.ConsumeLogs(ctx, batch().logs)
}

// NotifyClusterChange implements cluster.Component.
func (c *Component) NotifyClusterChange() {
	c.mu.RLock()
	enabled := c.args.Clustering.Enabled
	c.mu.RUnlock()
	if enabled {
		c.requestRestart()
	}
}

func (c *Component) requestRestart() {
	select {
	case c.restart <- struct{}{}:
	default:
	}
}
