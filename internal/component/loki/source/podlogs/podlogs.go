package podlogs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/oklog/run"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	commonk8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/component/loki/source/kubernetes"
	"github.com/grafana/alloy/internal/component/loki/source/kubernetes/kubetail"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.podlogs",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.podlogs
// component.
type Arguments struct {
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`

	// Client settings to connect to Kubernetes.
	Client commonk8s.ClientArguments `alloy:"client,block,optional"`

	Selector          config.LabelSelector `alloy:"selector,block,optional"`
	NamespaceSelector config.LabelSelector `alloy:"namespace_selector,block,optional"`
	TailFromEnd       bool                 `alloy:"tail_from_end,attr,optional"`

	// Node filtering settings to limit pod discovery to specific nodes.
	NodeFilter NodeFilterConfig `alloy:"node_filter,block,optional"`

	// PreserveDiscoveredLabels controls whether discovered Kubernetes meta labels
	// are preserved when forwarding logs to downstream components.
	PreserveDiscoveredLabels bool `alloy:"preserve_discovered_labels,attr,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`
}

// NodeFilterConfig configures node-based filtering for pod discovery.
// When enabled, only pods running on the specified node will be discovered,
// which is useful for DaemonSet deployments to avoid cross-node log collection.
type NodeFilterConfig struct {
	// Enabled controls whether node filtering is active.
	Enabled bool `alloy:"enabled,attr,optional"`
	// NodeName specifies the node name to filter by. If empty, the component
	// will attempt to use the NODE_NAME environment variable.
	NodeName string `alloy:"node_name,attr,optional"`
}

// DefaultArguments holds default settings for loki.source.kubernetes.
var DefaultArguments = Arguments{
	Client: commonk8s.DefaultClientArguments,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Component implements the loki.source.podlogs component.
type Component struct {
	log  log.Logger
	opts component.Options

	tailer     *kubetail.Manager
	reconciler *reconciler
	controller *controller

	positions positions.Positions
	handler   loki.LogsReceiver

	mut         sync.RWMutex
	args        Arguments
	lastOptions *kubetail.Options
	restConfig  *rest.Config

	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver
}

var (
	_ component.Component      = (*Component)(nil)
	_ component.DebugComponent = (*Component)(nil)
	_ cluster.Component        = (*Component)(nil)
)

// New creates a new loki.source.podlogs component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	positionsFile, err := positions.New(o.Logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: filepath.Join(o.DataPath, "positions.yml"),
	})
	if err != nil {
		return nil, err
	}

	data, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, err
	}

	var (
		tailer     = kubetail.NewManager(o.Logger, nil)
		reconciler = newReconciler(o.Logger, tailer, data.(cluster.Cluster))
		controller = newController(o.Logger, reconciler)
	)

	c := &Component{
		log:  o.Logger,
		opts: o,

		tailer:     tailer,
		reconciler: reconciler,
		controller: controller,

		positions: positionsFile,
		handler:   loki.NewLogsReceiver(),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer c.positions.Stop()

	defer func() {
		c.mut.RLock()
		defer c.mut.RUnlock()

		// Guard for safety, but it's not possible for Run to be called without
		// c.tailer being initialized.
		if c.tailer != nil {
			c.tailer.Stop()
		}
	}()

	var g run.Group

	g.Add(func() error {
		c.runHandler(ctx)
		return nil
	}, func(_ error) {
		cancel()
	})

	g.Add(func() error {
		err := c.controller.Run(ctx)
		if err != nil {
			level.Error(c.log).Log("msg", "controller exited with error", "err", err)
		}
		return err
	}, func(_ error) {
		cancel()
	})

	return g.Run()
}

func (c *Component) runHandler(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-c.handler.Chan():
			c.receiversMut.RLock()
			receivers := c.receivers
			c.receiversMut.RUnlock()

			for _, receiver := range receivers {
				receiver.Chan() <- entry
			}
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// Update the receivers before anything else, just in case something fails.
	c.receiversMut.Lock()
	c.receivers = newArgs.ForwardTo
	c.receiversMut.Unlock()

	c.mut.Lock()
	defer c.mut.Unlock()

	if err := c.updateTailer(newArgs); err != nil {
		return err
	}
	if err := c.updateReconciler(newArgs); err != nil {
		return err
	}
	if err := c.updateController(newArgs); err != nil {
		return err
	}

	c.args = newArgs
	return nil
}

// NotifyClusterChange implements cluster.Component.
func (c *Component) NotifyClusterChange() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if !c.args.Clustering.Enabled {
		return
	}
	c.controller.RequestReconcile()
}

// updateTailer updates the state of the tailer. mut must be held when calling.
func (c *Component) updateTailer(args Arguments) error {
	if reflect.DeepEqual(c.args.Client, args.Client) && c.lastOptions != nil {
		return nil
	}

	cfg, err := args.Client.BuildRESTConfig(c.log)
	if err != nil {
		return fmt.Errorf("building Kubernetes config: %w", err)
	}
	clientSet, err := kubeclient.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("building Kubernetes client: %w", err)
	}

	managerOpts := &kubetail.Options{
		Client:      clientSet,
		Handler:     loki.NewEntryHandler(c.handler.Chan(), func() {}),
		Positions:   c.positions,
		TailFromEnd: args.TailFromEnd,
	}
	c.lastOptions = managerOpts

	// Options changed; pass it to the tailer. This will never fail because it
	// only fails if the context gets canceled.
	//
	// TODO(rfratto): should we have a generous update timeout to prevent this
	// from potentially hanging forever?
	_ = c.tailer.UpdateOptions(context.Background(), managerOpts)
	return nil
}

// updateReconciler updates the state of the reconciler. This must only be
// called after updateTailer. mut must be held when calling.
func (c *Component) updateReconciler(args Arguments) error {
	// The clustering settings should always be updated,
	// even if the selectors haven't changed.
	c.reconciler.SetDistribute(args.Clustering.Enabled)

	var (
		selectorChanged                 = !reflect.DeepEqual(c.args.Selector, args.Selector)
		namespaceSelectorChanged        = !reflect.DeepEqual(c.args.NamespaceSelector, args.NamespaceSelector)
		nodeFilterChanged               = !reflect.DeepEqual(c.args.NodeFilter, args.NodeFilter)
		preserveDiscoveredLabelsChanged = c.args.PreserveDiscoveredLabels != args.PreserveDiscoveredLabels
	)

	// Update preserve discovered labels configuration
	c.reconciler.UpdatePreserveMetaLabels(args.PreserveDiscoveredLabels)

	// Update node filter configuration
	c.reconciler.UpdateNodeFilter(args.NodeFilter.Enabled, args.NodeFilter.NodeName)

	if !selectorChanged && !namespaceSelectorChanged && !nodeFilterChanged && !preserveDiscoveredLabelsChanged {
		return nil
	}

	sel, err := args.Selector.BuildSelector()
	if err != nil {
		return err
	}
	nsSel, err := args.NamespaceSelector.BuildSelector()
	if err != nil {
		return err
	}

	c.reconciler.UpdateSelectors(sel, nsSel)

	// Request a reconcile so the new selectors get applied.
	c.controller.RequestReconcile()
	return nil
}

// updateController updates the state of the controller. This must only be
// called after updateReconciler. mut must be held when calling.
func (c *Component) updateController(args Arguments) error {
	// We only need to update the controller if we already have a rest config
	// generated and our client args haven't changed since the last call.
	if reflect.DeepEqual(c.args.Client, args.Client) && c.restConfig != nil {
		return nil
	}

	cfg, err := args.Client.BuildRESTConfig(c.log)
	if err != nil {
		return fmt.Errorf("building Kubernetes config: %w", err)
	}
	c.restConfig = cfg

	return c.controller.UpdateConfig(cfg)
}

// DebugInfo returns debug information for loki.source.podlogs.
func (c *Component) DebugInfo() any {
	var info DebugInfo

	info.DiscoveredPodLogs = c.reconciler.DebugInfo()

	for _, target := range c.tailer.Targets() {
		var lastError string
		if err := target.LastError(); err != nil {
			lastError = err.Error()
		}

		info.Targets = append(info.Targets, kubernetes.DebugInfoTarget{
			Labels:          target.Labels().Map(),
			DiscoveryLabels: target.DiscoveryLabels().Map(),
			LastError:       lastError,
			UpdateTime:      target.LastEntry().Local(),
		})
	}

	return info
}

// DebugInfo stores debug information for loki.source.podlogs.
type DebugInfo struct {
	DiscoveredPodLogs []DiscoveredPodLogs          `alloy:"pod_logs,block"`
	Targets           []kubernetes.DebugInfoTarget `alloy:"target,block,optional"`
}
