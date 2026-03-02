package gcplog

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/gcplog/gcptypes"
	gt "github.com/grafana/alloy/internal/component/loki/source/gcplog/internal/gcplogtarget"
	"github.com/grafana/alloy/internal/util"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.gcplog",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.gcplog
// component.
type Arguments struct {
	PullTarget   *gcptypes.PullConfig `alloy:"pull,block,optional"`
	PushTarget   *gcptypes.PushConfig `alloy:"push,block,optional"`
	ForwardTo    []loki.LogsReceiver  `alloy:"forward_to,attr"`
	RelabelRules alloy_relabel.Rules  `alloy:"relabel_rules,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if (a.PullTarget != nil) == (a.PushTarget != nil) {
		return fmt.Errorf("exactly one of 'push' or 'pull' must be provided")
	}
	return nil
}

// Component implements the loki.source.gcplog component.
type Component struct {
	opts          component.Options
	metrics       *gt.Metrics
	serverMetrics *util.UncheckedCollector

	mut    sync.RWMutex
	fanout []loki.LogsReceiver
	target gt.Target

	handler loki.LogsReceiver
}

// New creates a new loki.source.gcplog component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:          o,
		metrics:       gt.NewMetrics(o.Registerer),
		handler:       loki.NewLogsReceiver(),
		fanout:        args.ForwardTo,
		serverMetrics: util.NewUncheckedCollector(nil),
	}

	o.Registerer.MustRegister(c.serverMetrics)

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		level.Info(c.opts.Logger).Log("msg", "loki.source.gcplog component shutting down, stopping the targets")
		c.mut.RLock()
		err := c.target.Stop()
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error while stopping gcplog target", "err", err)
		}
		c.mut.RUnlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.fanout {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.fanout = newArgs.ForwardTo

	var rcs []*relabel.Config
	if len(newArgs.RelabelRules) > 0 {
		rcs = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)
	}

	if c.target != nil {
		err := c.target.Stop()
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error while stopping gcplog target", "err", err)
		}
	}
	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})
	r := strings.NewReplacer(".", "_", "/", "_")
	jobName := r.Replace(c.opts.ID)

	if newArgs.PullTarget != nil {
		// TODO(@tpaschalis) Are there any options from "google.golang.org/api/option"
		// we should expose as configuration and pass here?
		t, err := gt.NewPullTarget(c.metrics, c.opts.Logger, entryHandler, jobName, newArgs.PullTarget, rcs)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create gcplog target with provided config", "err", err)
			return err
		}
		c.target = t
	}
	if newArgs.PushTarget != nil {
		// [gt.NewPushTarget] registers new metrics every time it is called. To
		// avoid issues with re-registering metrics with the same name, we create a
		// new registry for the target every time we create one, and pass it to an
		// unchecked collector to bypass uniqueness checking.
		registry := prometheus.NewRegistry()
		c.serverMetrics.SetCollector(registry)

		t, err := gt.NewPushTarget(c.metrics, c.opts.Logger, entryHandler, jobName, newArgs.PushTarget, rcs, registry)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create gcplog target with provided config", "err", err)
			return err
		}
		c.target = t
	}

	return nil
}

// DebugInfo returns information about the status of targets.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return targetDebugInfo{Details: c.target.Details()}
}

type targetDebugInfo struct {
	Details map[string]string `alloy:"target_info,attr"`
}
