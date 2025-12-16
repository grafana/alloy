package cloudflare

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.cloudflare",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the
// loki.source.cloudflare component.
type Arguments struct {
	APIToken         alloytypes.Secret   `alloy:"api_token,attr"`
	ZoneID           string              `alloy:"zone_id,attr"`
	Labels           map[string]string   `alloy:"labels,attr,optional"`
	Workers          int                 `alloy:"workers,attr,optional"`
	PullRange        time.Duration       `alloy:"pull_range,attr,optional"`
	FieldsType       FieldsType          `alloy:"fields_type,attr,optional"`
	AdditionalFields []string            `alloy:"additional_fields,attr,optional"`
	ForwardTo        []loki.LogsReceiver `alloy:"forward_to,attr"`
}

func (c Arguments) tailerConfig() *tailerConfig {
	lbls := make(model.LabelSet, len(c.Labels))
	for k, v := range c.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}
	return &tailerConfig{
		APIToken:         string(c.APIToken),
		ZoneID:           c.ZoneID,
		Labels:           lbls,
		Workers:          c.Workers,
		PullRange:        model.Duration(c.PullRange),
		FieldsType:       c.FieldsType,
		AdditionalFields: c.AdditionalFields,
	}
}

// DefaultArguments sets the configuration defaults.
var DefaultArguments = Arguments{
	Workers:    3,
	PullRange:  1 * time.Minute,
	FieldsType: FieldsTypeDefault,
}

// SetToDefault implements syntax.Defaulter.
func (c *Arguments) SetToDefault() {
	*c = DefaultArguments
}

// Validate implements syntax.Validator.
func (c *Arguments) Validate() error {
	if c.PullRange < 0 {
		return fmt.Errorf("pull_range must be a positive duration")
	}
	return nil
}

// Component implements the loki.source.cloudflare component.
type Component struct {
	opts    component.Options
	posFile positions.Positions
	handler loki.LogsReceiver
	metrics *metrics

	// mut is used to protect access to tailer.
	mut    sync.RWMutex
	tailer *tailer

	fanout *loki.Fanout
}

// New creates a new loki.source.cloudflare component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	positionsFile, err := positions.New(o.Logger, positions.Config{
		SyncPeriod:        10 * time.Second,
		PositionsFile:     filepath.Join(o.DataPath, "positions.yml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:    o,
		metrics: newMetrics(o.Registerer),
		handler: loki.NewLogsReceiver(),
		fanout:  loki.NewFanout(args.ForwardTo),
		posFile: positionsFile,
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
		level.Info(c.opts.Logger).Log("msg", "loki.source.cloudflare component shutting down")

		// NOTE: We need to stop posFile first so we don't record entries we are draining.
		c.posFile.Stop()
		source.Drain(c.handler, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			c.tailer.stop()
		})
	}()

	source.Consume(ctx, c.handler, c.fanout)
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	if c.tailer != nil {
		c.tailer.stop()
	}

	t, err := newTailer(c.metrics, c.opts.Logger, c.handler, c.posFile, newArgs.tailerConfig())
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create cloudflare target with provided config", "err", err)
		return err
	}
	c.tailer = t

	return nil
}

// DebugInfo returns information about the status of targets.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()

	return targetDebugInfo{
		Ready:   c.tailer.ready(),
		Details: c.tailer.details(),
	}
}

type targetDebugInfo struct {
	Ready   bool              `alloy:"ready,attr"`
	Details map[string]string `alloy:"target_info,attr"`
}
