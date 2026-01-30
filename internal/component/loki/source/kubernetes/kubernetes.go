// Package kubernetes implements the loki.source.kubernetes component.
package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/go-kit/log"
	"k8s.io/client-go/kubernetes"

	"github.com/grafana/alloy/internal/component"
	commonk8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/component/loki/source/kubernetes/kubetail"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.kubernetes",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.kubernetes
// component.
type Arguments struct {
	Targets   []discovery.Target  `alloy:"targets,attr"`
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`

	// Client settings to connect to Kubernetes.
	Client commonk8s.ClientArguments `alloy:"client,block,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`
}

// DefaultArguments holds default settings for loki.source.kubernetes.
var DefaultArguments = Arguments{
	Client: commonk8s.DefaultClientArguments,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Component implements the loki.source.kubernetes component.
type Component struct {
	log       log.Logger
	opts      component.Options
	positions positions.Positions
	cluster   cluster.Cluster

	mut         sync.Mutex
	args        Arguments
	tailer      *kubetail.Manager
	lastOptions *kubetail.Options

	handler loki.LogsReceiver

	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver
}

var (
	_ component.Component      = (*Component)(nil)
	_ component.DebugComponent = (*Component)(nil)
	_ cluster.Component        = (*Component)(nil)
)

// New creates a new loki.source.kubernetes component.
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

	c := &Component{
		cluster:   data.(cluster.Cluster),
		log:       o.Logger,
		opts:      o,
		handler:   loki.NewLogsReceiver(),
		positions: positionsFile,
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.positions.Stop()

	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()

		// Guard for safety, but it's not possible for Run to be called without
		// c.tailer being initialized.
		if c.tailer != nil {
			c.tailer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
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

	managerOpts, err := c.getTailerOptions(newArgs)
	if err != nil {
		return err
	}

	switch {
	case c.tailer == nil:
		// First call to Update; build the tailer.
		c.tailer = kubetail.NewManager(c.log, managerOpts)

	case managerOpts != c.lastOptions:
		// Options changed; pass it to the tailer.
		//
		// This will never fail because it only fails if the context gets canceled.
		//
		// TODO(rfratto): should we have a generous update timeout to prevent this
		// from potentially hanging forever?
		_ = c.tailer.UpdateOptions(context.Background(), managerOpts)
		c.lastOptions = managerOpts

	default:
		// No-op: manager already exists and options didn't change.
	}

	c.resyncTargets(newArgs.Targets)
	c.args = newArgs
	return nil
}

func (c *Component) resyncTargets(targets []discovery.Target) {
	distTargets := discovery.NewDistributedTargetsWithCustomLabels(c.args.Clustering.Enabled, c.cluster, targets, kubetail.ClusteringLabels)
	targets = distTargets.LocalTargets()

	tailTargets := make([]*kubetail.Target, 0, len(targets))
	for _, target := range targets {
		lset := target.PromLabels()
		processed, err := kubetail.PrepareLabels(lset, c.opts.ID)
		if err != nil {
			// TODO(rfratto): should this set the health of the component?
			level.Error(c.log).Log("msg", "failed to process input target", "target", lset.String(), "err", err)
			continue
		}
		tailTargets = append(tailTargets, kubetail.NewTarget(lset, processed, false))
	}

	// This will never fail because it only fails if the context gets canceled.
	//
	// TODO(rfratto): should we have a generous update timeout to prevent this
	// from potentially hanging forever?
	_ = c.tailer.SyncTargets(context.Background(), tailTargets)
}

// NotifyClusterChange implements cluster.Component.
func (c *Component) NotifyClusterChange() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if !c.args.Clustering.Enabled {
		return
	}
	c.resyncTargets(c.args.Targets)
}

// getTailerOptions gets tailer options from arguments. If args hasn't changed
// from the last call to getTailerOptions, c.lastOptions is returned.
// c.lastOptions must be updated by the caller.
//
// getTailerOptions must only be called when c.mut is held.
func (c *Component) getTailerOptions(args Arguments) (*kubetail.Options, error) {
	if reflect.DeepEqual(c.args.Client, args.Client) && c.lastOptions != nil {
		return c.lastOptions, nil
	}

	cfg, err := args.Client.BuildRESTConfig(c.log)
	if err != nil {
		return c.lastOptions, fmt.Errorf("building Kubernetes config: %w", err)
	}
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return c.lastOptions, fmt.Errorf("building Kubernetes client: %w", err)
	}

	return &kubetail.Options{
		Client:    clientSet,
		Handler:   loki.NewEntryHandler(c.handler.Chan(), func() {}),
		Positions: c.positions,
	}, nil
}

// DebugInfo returns debug information for loki.source.kubernetes.
func (c *Component) DebugInfo() any {
	var info DebugInfo

	for _, target := range c.tailer.Targets() {
		var lastError string
		if err := target.LastError(); err != nil {
			lastError = err.Error()
		}

		info.Targets = append(info.Targets, DebugInfoTarget{
			Labels:          target.Labels().Map(),
			DiscoveryLabels: target.DiscoveryLabels().Map(),
			LastError:       lastError,
			UpdateTime:      target.LastEntry().Local(),
		})
	}

	return info
}

// DebugInfo represents debug information for loki.source.kubernetes.
type DebugInfo struct {
	Targets []DebugInfoTarget `alloy:"target,block,optional"`
}

// DebugInfoTarget is debug information for an individual target being tailed
// for logs.
type DebugInfoTarget struct {
	Labels          map[string]string `alloy:"labels,attr,optional"`
	DiscoveryLabels map[string]string `alloy:"discovery_labels,attr,optional"`
	LastError       string            `alloy:"last_error,attr,optional"`
	UpdateTime      time.Time         `alloy:"update_time,attr,optional"`
}
