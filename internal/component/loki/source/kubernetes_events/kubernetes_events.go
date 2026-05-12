// Package kubernetes_events implements the loki.source.kubernetes_events
// component.
package kubernetes_events

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/shard"
	"k8s.io/client-go/rest"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
)

// Generous timeout period for configuring informers
const informerSyncTimeout = 10 * time.Minute

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.kubernetes_events",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the
// loki.source.kubernetes_events component.
type Arguments struct {
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`

	JobName    string   `alloy:"job_name,attr,optional"`
	Namespaces []string `alloy:"namespaces,attr,optional"`
	LogFormat  string   `alloy:"log_format,attr,optional"`

	// Client settings to connect to Kubernetes.
	Client   kubernetes.ClientArguments `alloy:"client,block,optional"`
	Position positions.Config           `alloy:"position,block,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`
}

// DefaultArguments holds default settings for loki.source.kubernetes_events.
var DefaultArguments = Arguments{
	JobName:   "loki.source.kubernetes_events",
	LogFormat: logFormatFmt,
	Client:    kubernetes.DefaultClientArguments,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.JobName = "loki.source.kubernetes_events"
	args.LogFormat = logFormatFmt
	args.Client.SetToDefault()
	args.Position.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.JobName == "" {
		return fmt.Errorf("job_name must not be an empty string")
	}
	if args.LogFormat != logFormatFmt && args.LogFormat != logFormatJson {
		return fmt.Errorf("supported values of log_format are %s and %s", logFormatFmt, logFormatJson)
	}
	return nil
}

// Component implements the loki.source.kubernetes_events component, which
// watches events from Kubernetes and forwards received events to other Loki
// components.
type Component struct {
	log       log.Logger
	opts      component.Options
	positions positions.Positions
	handler   loki.LogsReceiver
	cluster   cluster.Cluster

	mut        sync.RWMutex
	args       Arguments
	restConfig *rest.Config
	scheduler  *source.Scheduler[string]

	fanout *loki.Fanout
}

var (
	_ component.Component      = (*Component)(nil)
	_ component.DebugComponent = (*Component)(nil)
	_ cluster.Component        = (*Component)(nil)
)

// New creates a new loki.source.kubernetes_events component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	positionsFile, err := positions.New(
		o.Logger,
		filepath.Join(o.DataPath, "positions.yml"),
		args.Position,
	)
	if err != nil {
		return nil, err
	}

	data, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		log:       o.Logger,
		opts:      o,
		positions: positionsFile,
		handler:   loki.NewLogsReceiver(),
		cluster:   data.(cluster.Cluster),
		scheduler: source.NewScheduler[string](),
		fanout:    loki.NewFanout(args.ForwardTo),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		defer c.positions.Stop()
		loki.Drain(c.handler, c.fanout, loki.DefaultDrainTimeout, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			c.scheduler.Stop()
		})
	}()

	loki.Consume(ctx, c.handler, c.fanout)
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	c.positions.Update(newArgs.Position)
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	// Create a new restConfig if we don't have one or if our arguments changed.
	if c.restConfig == nil || !reflect.DeepEqual(c.args.Client, newArgs.Client) {
		var err error
		c.restConfig, err = newArgs.Client.BuildRESTConfig(c.log)
		if err != nil {
			return fmt.Errorf("building Kubernetes client config: %w", err)
		}

		// When restConfig changes we need to restart all scheduled sources.
		c.scheduler.Reset()
	}

	c.args = newArgs
	c.reconcile()
	return nil
}

// reconcile synchronizes the running event controllers with the desired set
// of namespaces, filtered by clustering ownership.
func (c *Component) reconcile() {
	source.Reconcile(
		c.opts.Logger,
		c.scheduler,
		c.localNamespaces(),
		func(namespace string) string { return namespace },
		func(_ string, namespace string) (source.Source[string], error) {
			return newEventController(eventControllerOptions{
				Log:          c.log,
				Config:       c.restConfig,
				Namespace:    namespace,
				JobName:      c.args.JobName,
				InstanceName: c.opts.ID,
				Receiver:     c.handler,
				Positions:    c.positions,
				LogFormat:    c.args.LogFormat,
			}), nil
		},
	)
}

// NotifyClusterChange implements cluster.Component.
func (c *Component) NotifyClusterChange() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if !c.args.Clustering.Enabled {
		return
	}
	c.reconcile()
}

// localNamespaces returns an iterator of namespaces that this node should
// watch, filtered by cluster ownership when clustering is enabled.
func (c *Component) localNamespaces() iter.Seq[string] {
	return func(yield func(string) bool) {
		if c.args.Clustering.Enabled && !c.cluster.Ready() {
			// When clustering is enabled but the cluster isn't ready yet,
			// don't watch any namespaces. NotifyClusterChange will be called
			// once the cluster is ready, triggering a reconcile.
			return
		}

		for ns := range getNamespaces(c.args) {
			if c.args.Clustering.Enabled {
				// Use the namespace name as the hash key. For the "all namespaces"
				// case (empty string), this results in a single key, so only one
				// node in the cluster will watch all events.
				peers, err := c.cluster.Lookup(shard.StringKey(ns), 1, shard.OpReadWrite)
				if err == nil && len(peers) > 0 && !peers[0].Self {
					continue // This namespace belongs to another node.
				}
			}
			if !yield(ns) {
				return
			}
		}
	}
}

// getNamespaces returns an iterator of namespaces to watch from the arguments. If the
// list of namespaces is empty, returns an iterator to watch all namespaces.
func getNamespaces(args Arguments) iter.Seq[string] {
	return func(yield func(string) bool) {
		if len(args.Namespaces) == 0 {
			// Empty string means to watch all namespaces
			yield("")
			return
		}

		for _, namespace := range args.Namespaces {
			if !yield(namespace) {
				return
			}
		}
	}
}

// DebugInfo implements [component.DebugComponent].
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()

	type Info struct {
		Controllers []controllerInfo `alloy:"event_controller,block,optional"`
	}

	var info Info
	for s := range c.scheduler.Sources() {
		info.Controllers = append(info.Controllers, s.(*eventController).DebugInfo())
	}

	return info
}
