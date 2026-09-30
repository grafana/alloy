// Package k8s_workloads provides the otelcol.receiver.k8s_workloads component.
package k8s_workloads

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
		Name:      "otelcol.receiver.k8s_workloads",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures otelcol.receiver.k8s_workloads.
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

// Component watches Kubernetes Deployment rollout transitions.
type Component struct {
	opts    component.Options
	cluster cluster.Cluster

	mu                 sync.RWMutex
	args               Arguments
	restConfig         *rest.Config
	logsSink           consumer.Logs
	restart            chan struct{}
	clusterChanged     chan struct{}
	clusterLastChanged time.Time
}

var (
	_ component.Component = (*Component)(nil)
	_ cluster.Component   = (*Component)(nil)
)

// New creates a new otelcol.receiver.k8s_workloads component.
func New(opts component.Options, args Arguments) (*Component, error) {
	clusterData, err := opts.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("getting cluster service: %w", err)
	}

	c := &Component{
		opts:           opts,
		cluster:        clusterData.(cluster.Cluster),
		restart:        make(chan struct{}, 1),
		clusterChanged: make(chan struct{}, 1),
	}
	if err := c.update(args, false); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
generationLoop:
	for {
		c.mu.RLock()
		args := c.args
		restConfig := rest.CopyConfig(c.restConfig)
		c.mu.RUnlock()

		watch, ownershipErr := c.shouldWatch(args)
		generationCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			if ownershipErr != nil {
				done <- ownershipErr
				return
			}
			done <- c.runGeneration(generationCtx, args, restConfig, watch)
		}()

		for {
			select {
			case <-ctx.Done():
				cancel()
				<-done
				return nil
			case <-c.restart:
				cancel()
				<-done
				continue generationLoop
			case <-c.clusterChanged:
				nextWatch, err := c.shouldWatch(args)
				if err != nil {
					c.requestRestart()
					continue
				}
				if nextWatch == watch {
					continue
				}
				cancel()
				<-done
				continue generationLoop
			case err := <-done:
				cancel()
				if err == nil || ctx.Err() != nil {
					return nil
				}
				c.opts.Logger.Error("Kubernetes workload watcher stopped; retrying", "err", err)
				timer := time.NewTimer(time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil
				case <-c.restart:
					timer.Stop()
				case <-c.clusterChanged:
					timer.Stop()
				case <-timer.C:
				}
				continue generationLoop
			}
		}
	}
}

func (c *Component) shouldWatch(args Arguments) (bool, error) {
	if args.Clustering.Enabled {
		if !c.cluster.Ready() {
			return false, nil
		}
		owners, err := c.cluster.Lookup(shard.StringKey(c.opts.ID), 1, shard.OpReadWrite)
		if err != nil {
			return false, fmt.Errorf("determining workload watcher ownership: %w", err)
		}
		if len(owners) != 1 || !owners[0].Self {
			return false, nil
		}
	}
	return true, nil
}

func (c *Component) runGeneration(ctx context.Context, args Arguments, restConfig *rest.Config, watch bool) error {
	if !watch {
		<-ctx.Done()
		return nil
	}

	if args.Clustering.Enabled && c.cluster.Enabled() {
		if err := c.waitForClusterStability(ctx); err != nil {
			return err
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
	if err := args.Validate(); err != nil {
		return err
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
	c.mu.Lock()
	enabled := c.args.Clustering.Enabled
	c.clusterLastChanged = time.Now()
	c.mu.Unlock()
	if enabled {
		select {
		case c.clusterChanged <- struct{}{}:
		default:
		}
	}
}

func (c *Component) requestRestart() {
	select {
	case c.restart <- struct{}{}:
	default:
	}
}

// Let gossip converge before starting a new watcher. Ownership
// loss still cancels this wait immediately through the generation context. Bound
// the delay so continuous node churn cannot indefinitely prevent watching.
func (c *Component) waitForClusterStability(ctx context.Context) error {
	const quietPeriod = 30 * time.Second
	start := time.Now()
	deadline := start.Add(90 * time.Second)
	for {
		c.mu.RLock()
		changed := c.clusterLastChanged
		c.mu.RUnlock()
		if changed.Before(start) {
			changed = start
		}
		ready := changed.Add(quietPeriod)
		if ready.After(deadline) {
			ready = deadline
		}
		delay := time.Until(ready)
		if delay <= 0 {
			return ctx.Err()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
