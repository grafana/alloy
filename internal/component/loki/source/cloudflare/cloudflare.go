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
	cft "github.com/grafana/alloy/internal/component/loki/source/cloudflare/internal/cloudflaretarget"
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
	FieldsType       string              `alloy:"fields_type,attr,optional"`
	AdditionalFields []string            `alloy:"additional_fields,attr,optional"`
	ForwardTo        []loki.LogsReceiver `alloy:"forward_to,attr"`
}

// Convert returns a cloudflaretarget Config struct from the Arguments.
func (c Arguments) Convert() *cft.Config {
	lbls := make(model.LabelSet, len(c.Labels))
	for k, v := range c.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}
	return &cft.Config{
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
	FieldsType: string(cft.FieldsTypeDefault),
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
	_, err := cft.Fields(cft.FieldsType(c.FieldsType), c.AdditionalFields)
	if err != nil {
		return fmt.Errorf("invalid fields_type set; the available values are 'default', 'minimal', 'extended', 'custom' and 'all'")
	}
	return nil
}

// Component implements the loki.source.cloudflare component.
type Component struct {
	opts    component.Options
	metrics *cft.Metrics

	mut    sync.RWMutex
	fanout []loki.LogsReceiver
	target *cft.Target

	posFile positions.Positions
	handler loki.LogsReceiver
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
		metrics: cft.NewMetrics(o.Registerer),
		handler: loki.NewLogsReceiver(),
		fanout:  args.ForwardTo,
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
		c.mut.RLock()
		level.Info(c.opts.Logger).Log("msg", "loki.source.cloudflare component shutting down, stopping the target")
		c.target.Stop()
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

	if c.target != nil {
		c.target.Stop()
	}
	entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})

	t, err := cft.NewTarget(c.metrics, c.opts.Logger, entryHandler, c.posFile, newArgs.Convert())
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to create cloudflare target with provided config", "err", err)
		return err
	}
	c.target = t

	return nil
}

// DebugInfo returns information about the status of targets.
func (c *Component) DebugInfo() interface{} {
	c.mut.RLock()
	defer c.mut.RUnlock()

	lbls := make(map[string]string, len(c.target.Labels()))
	for k, v := range c.target.Labels() {
		lbls[string(k)] = string(v)
	}
	return targetDebugInfo{
		Ready:   c.target.Ready(),
		Details: c.target.Details(),
	}
}

type targetDebugInfo struct {
	Ready   bool              `alloy:"ready,attr"`
	Details map[string]string `alloy:"target_info,attr"`
}
