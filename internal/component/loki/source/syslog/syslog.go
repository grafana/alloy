package syslog

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	scrapeconfig "github.com/grafana/alloy/internal/component/loki/source/syslog/config"
	st "github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.syslog",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.syslog
// component.
type Arguments struct {
	SyslogListeners []ListenerConfig    `alloy:"listener,block"`
	ForwardTo       []loki.LogsReceiver `alloy:"forward_to,attr"`
	RelabelRules    alloy_relabel.Rules `alloy:"relabel_rules,attr,optional"`
}

// Component implements the loki.source.syslog component.
type Component struct {
	opts    component.Options
	metrics *st.Metrics

	mut     sync.RWMutex
	args    Arguments
	fanout  []loki.LogsReceiver
	targets []*st.SyslogTarget

	targetsUpdated chan struct{}
	handler        loki.LogsReceiver
}

// New creates a new loki.source.syslog component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:           o,
		metrics:        st.NewMetrics(o.Registerer),
		handler:        loki.NewLogsReceiver(),
		fanout:         args.ForwardTo,
		targetsUpdated: make(chan struct{}, 1),
		targets:        []*st.SyslogTarget{},
	}

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		// Start draining routine to prevent potential deadlock if targets attempt to send during Stop().
		cancel := c.startDrainingRoutine()
		defer cancel()

		// Stop all targets
		c.mut.RLock()
		defer c.mut.RUnlock()
		level.Info(c.opts.Logger).Log("msg", "loki.source.syslog component shutting down, stopping listeners")
		for _, l := range c.targets {
			err := l.Stop()
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "error while stopping syslog listener", "err", err)
			}
		}
	}()

	for {
		select {
		case <-c.targetsUpdated:
			c.reloadTargets()
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
	if err := c.checkExperimentalFeatures(newArgs); err != nil {
		return err
	}

	prevArgs := c.args
	c.fanout = newArgs.ForwardTo

	c.args = newArgs

	if listenersChanged(prevArgs.SyslogListeners, newArgs.SyslogListeners) || relabelRulesChanged(prevArgs.RelabelRules, newArgs.RelabelRules) {
		// trigger targets update
		select {
		case c.targetsUpdated <- struct{}{}:
		default:
		}
	}

	return nil
}

func (c *Component) checkExperimentalFeatures(args Arguments) error {
	isExperimental := c.opts.MinStability.Permits(featuregate.StabilityExperimental)
	if isExperimental {
		return nil
	}

	for _, listener := range args.SyslogListeners {
		if listener.SyslogFormat == scrapeconfig.SyslogFormatRaw {
			return fmt.Errorf("%q syslog format is available only at experimental stability level", scrapeconfig.SyslogFormatRaw)
		}

		if listener.RFC3164CiscoComponents != nil {
			return errors.New("rfc3164_cisco_components block is available only at experimental stability level")
		}
	}

	return nil
}

func (c *Component) startDrainingRoutine() func() {
	readCtx, cancel := context.WithCancel(context.Background())
	c.mut.RLock()
	defer c.mut.RUnlock()
	fanoutCopy := make([]loki.LogsReceiver, len(c.fanout))
	copy(fanoutCopy, c.fanout)
	go func() {
		for {
			select {
			case <-readCtx.Done():
				return
			case entry := <-c.handler.Chan():
				for _, receiver := range fanoutCopy {
					receiver.Chan() <- entry
				}
			}
		}
	}()
	return cancel
}

func (c *Component) reloadTargets() {
	// Start draining routine to prevent potential deadlock if targets attempt to send during Stop().
	cancel := c.startDrainingRoutine()

	// Grab current state
	c.mut.RLock()
	var rcs []*relabel.Config
	if len(c.args.RelabelRules) > 0 {
		rcs = alloy_relabel.ComponentToPromRelabelConfigs(c.args.RelabelRules)
	}
	targetsToStop := make([]*st.SyslogTarget, len(c.targets))
	copy(targetsToStop, c.targets)
	c.mut.RUnlock()

	// Stop existing targets
	for _, l := range targetsToStop {
		err := l.Stop()
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error while stopping syslog listener", "err", err)
		}
	}

	// Stop draining routine
	cancel()

	// Create new targets
	c.mut.Lock()
	defer c.mut.Unlock()
	c.targets = make([]*st.SyslogTarget, 0)
	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})

	for _, cfg := range c.args.SyslogListeners {
		promtailCfg, cfgErr := cfg.Convert()
		if cfgErr != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to convert syslog listener config", "err", cfgErr)
			continue
		}

		t, err := st.NewSyslogTarget(c.metrics, c.opts.Logger, entryHandler, rcs, promtailCfg)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create syslog listener with provided config", "err", err)
			continue
		}
		c.targets = append(c.targets, t)
	}
}

// DebugInfo returns information about the status of listeners.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()
	var res readerDebugInfo

	for _, t := range c.targets {
		res.ListenersInfo = append(res.ListenersInfo, listenerInfo{
			Ready:         t.Ready(),
			ListenAddress: t.ListenAddress().String(),
			Labels:        t.Labels().String(),
		})
	}
	return res
}

type readerDebugInfo struct {
	ListenersInfo []listenerInfo `alloy:"listeners_info,attr"`
}

type listenerInfo struct {
	Ready         bool   `alloy:"ready,attr"`
	ListenAddress string `alloy:"listen_address,attr"`
	Labels        string `alloy:"labels,attr"`
}

func listenersChanged(prev, next []ListenerConfig) bool {
	return !reflect.DeepEqual(prev, next)
}

func relabelRulesChanged(prev, next alloy_relabel.Rules) bool {
	return !reflect.DeepEqual(prev, next)
}
